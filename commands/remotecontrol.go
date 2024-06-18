package commands

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"transformer/cmdr"
	"transformer/grubutils"
)

type CommandTag int

const (
	CommandTerminate CommandTag = iota
	CommandCli
	CommandSetupCanaryBoot
)

type Command struct {
	Tag     CommandTag
	Command string
	Args    []string
	UUID    string
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

func (c *Commands) handleSetupCanaryBootCommand(uuid string, enc *gob.Encoder) {
	var err error
	reportError := func(err error) {
		err1 := enc.Encode(Reply{
			Tag: ReplyError,
			Err: err.Error(),
		})
		if err1 != nil {
			c.Errorf("Error encoding reply: %+v", err1)
		}
	}
	defer func() {
		if r := recover(); r != nil {
			c.Errorf("Recovered from panic: %+v", r)
			reportError(fmt.Errorf("recovered from panic: %+v", r))
			return
		}
		if err != nil {
			reportError(err)
			return
		}
		err1 := enc.Encode(Reply{
			Tag: ReplyCommandCompleted,
		})
		if err1 != nil {
			c.Errorf("Error encoding reply: %+v", err1)
		}
	}()
	err = enc.Encode(Reply{
		Tag: ReplyCommandStarted,
	})
	if err != nil {
		c.Errorf("Error encoding reply: %+v", err)
		return
	}
	grubcfgf, err := os.Open("/boot/grub/grub.cfg")
	if err != nil {
		c.Errorf("Error: could not read the GRUB configuration: %v", err)
		return
	}
	defer grubcfgf.Close()
	grubcfg, err := io.ReadAll(grubcfgf)
	if err != nil {
		c.Errorf("Error: could not read the GRUB configuration: %v", err)
		return
	}
	c.Infof("Parsing GRUB configuration")
	menus, _, err := grubutils.GrubParser(string(grubcfg))
	if err != nil {
		c.Errorf("Error: could not parse the GRUB configuration: %v", err)
		c.Errorf("Given:\n %s", string(grubcfg))
		return
	}
	// Find the menu entry with the UUID of the new OS partition
	var newOSMenuEntry grubutils.MenuEntry
	for _, m := range menus {
		if strings.Contains(strings.ToLower(m.Id), uuid) {
			newOSMenuEntry = m
		}
	}
	if newOSMenuEntry.Id == "" {
		c.Errorf("Error: could not find the new OS partition in the GRUB configuration. Looking for %s", uuid)
		err = fmt.Errorf("error: could not find the new OS partition in the GRUB configuration. Looking for %s", uuid)
		return
	}
	c.Infof("Found the menu entry for the new OS partition: %s", newOSMenuEntry.Id)
	grCmd := c.Cmd("grub-reboot", newOSMenuEntry.Id)
	err = c.RunCmd(grCmd)
	if err != nil {
		c.Errorf("Error running grub-reboot: %v", err)
		err = fmt.Errorf("error running grub-reboot: %v", err)
		return
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
		case CommandSetupCanaryBoot:
			c.handleSetupCanaryBootCommand(cmd.UUID, enc)
		}

	}
}
