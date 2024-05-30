package commands

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"transformer/cmdr"
)

type CommandTag int

const (
	CommandTerminate CommandTag = iota
	CommandCli
)

type Command struct {
	Tag     CommandTag
	Command string
	Args    []string
	StdIn   string
}

type ReplyTag int

const (
	ReplyError ReplyTag = iota
	ReplyCommandStarted
	ReplyCommandCompleted
)

type Reply struct {
	Tag    ReplyTag
	Err    string
	PID    int
	StdOut string
	StdErr string
}

func (c *Commands) StartRemoteControlSession(socketPath string) (err error) {
	return c.WithGroupInScopeErr("StartRemoteControlSession", func(c *Commands) (err error) {
		wg := &sync.WaitGroup{}
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			c.Errorf("Error setting up listener: %v", err)
			return
		}
	stopAcceptingConnections:
		for {
			conn, err := listener.Accept()
			if err != nil {
				c.Errorf("Error accepting connection: %v", err)
				break stopAcceptingConnections
			}
			wg.Add(1)
			go c.handleConnection(conn, listener, wg)
		}

		wg.Wait()
		return nil
	})

}

func (c *Commands) handleCliCommand(cmd Command, enc *gob.Encoder) {
	cliCmd := c.Cmd(cmd.Command, cmd.Args...)
	if cmd.StdIn != "" {
		bb := strings.NewReader(cmd.StdIn)
		cliCmd.SetStdin(bb)
	}
	outputs := c.RunCmdAsync(cliCmd)
	for output := range outputs {
		switch output.Tag {
		case cmdr.ARStarted:
			c.Infof("Command started with PID: %d", output.PID)
			err := enc.Encode(Reply{Tag: ReplyCommandStarted, PID: output.PID})
			if err != nil {
				c.Errorf("Error encoding reply: %+v", err)
				continue
			}
		case cmdr.ARCompleted:
			c.Infof("Command completed with PID: %d", output.PID)
			err := enc.Encode(Reply{
				Tag:    ReplyCommandCompleted,
				PID:    output.PID,
				StdOut: cliCmd.GetStdoutAsString(),
				StdErr: cliCmd.GetStderrAsString(),
			})
			if err != nil {
				c.Errorf("Error encoding reply: %+v", err)
				continue
			}
		case cmdr.ARError:
			c.Errorf("Got an error from command: %+v", output.Error)
			err := enc.Encode(Reply{
				Tag:    ReplyError,
				Err:    output.Error.Error(),
				StdOut: cliCmd.GetStdoutAsString(),
				StdErr: cliCmd.GetStderrAsString(),
			})
			if err != nil {
				c.Errorf("Error encoding reply: %+v", err)
				continue
			}
		}
	}
}

func (c *Commands) handleConnection(conn net.Conn, listener net.Listener, wg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		wg.Done()
	}()

	enc := gob.NewEncoder(conn) // Will write to network.
	dec := gob.NewDecoder(conn)

	for {
		var cmd Command
		c.Infof("Waiting for command...")
		err := dec.Decode(&cmd)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			err = fmt.Errorf("error decoding command: %+v", err)
			c.Errorf("Error decoding command: %+v", err)
			reply := Reply{Tag: ReplyError, Err: err.Error()}
			err = enc.Encode(reply)
			if err != nil {
				c.Errorf("Error sending reply: %+v", err)
				c.Errorf("Terminating connection")
				return
			}
			continue
		}
		c.Infof("Decoded command: %+v", cmd)

		switch cmd.Tag {
		case CommandTerminate:
			listener.Close()
			return
		case CommandCli:
			c.handleCliCommand(cmd, enc)
		}

	}
}
