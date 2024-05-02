package cmdr

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type subcommandStdErrEvent struct {
	command string
	data    []byte
}

type subcommandStdErr struct {
	command string
	sink    chan subcommandStdErrEvent
}

func (s subcommandStdErr) Write(p []byte) (n int, err error) {
	s.sink <- subcommandStdErrEvent{
		command: s.command,
		data:    p,
	}
	return len(p), nil
}

type pipedCommands struct {
	commands  []Command
	log       *slog.Logger
	exitError *exec.ExitError
	exitCode  int
	stderr    io.Writer
	wg        sync.WaitGroup
	sink      chan subcommandStdErrEvent
}

func PipeCommands(commands ...Command) Command {
	return &pipedCommands{
		commands: commands,
	}
}

func (c *pipedCommands) Running() bool {
	for _, cmd := range c.commands {
		if !cmd.Running() {
			return false
		}
	}
	return true
}

func (c *pipedCommands) String() string {
	subcommands := make([]string, 0, len(c.commands))
	for _, cmd := range c.commands {
		subcommands = append(subcommands, cmd.String())
	}
	return strings.Join(subcommands, " | ")
}

func (c *pipedCommands) GetExitCode() int {
	return c.exitCode
}

func (c *pipedCommands) GetExitError() *exec.ExitError {
	return c.exitError
}

func (c *pipedCommands) GetStdoutAsString() string {
	return c.commands[len(c.commands)-1].GetStdoutAsString()
}

func (c *pipedCommands) GetStderrAsString() string {
	if bb, ok := c.stderr.(*bytes.Buffer); ok {
		return bb.String()
	}
	return ""
}

func (c *pipedCommands) SetStdin(stdin io.Reader) {
	c.commands[0].SetStdin(stdin)
}

func (c *pipedCommands) SetStdout(stdout io.Writer) {
	c.commands[len(c.commands)-1].SetStdout(stdout)
}

func (c *pipedCommands) SetStderr(stderr io.Writer) {
	c.stderr = stderr
}

func (c *pipedCommands) SetUnifiedOutput(sink func(UnifiedCommandOuput) error) {
	c.commands[len(c.commands)-1].SetStdout(&unifiedOutputSink{
		typ:  Stdout,
		sink: sink,
	})
	c.stderr = &unifiedOutputSink{
		typ:  Stderr,
		sink: sink,
	}
}

func (c *pipedCommands) SetInteractiveSink(interactiveSink InteractiveSink) {
	c.commands[0].SetStdin(interactiveSink.Responder())
	c.commands[len(c.commands)-1].SetStdout(&unifiedOutputSink{
		typ:  Stdout,
		sink: interactiveSink.Sink,
	})
	c.stderr = &unifiedOutputSink{
		typ:  Stderr,
		sink: interactiveSink.Sink,
	}
}

func (c *pipedCommands) SetLogger(logger *slog.Logger) {
	c.log = logger
	for _, cmd := range c.commands {
		cmd.SetLogger(logger.WithGroup(cmd.String()))
	}
}

func (c *pipedCommands) PipeTo(command Command) error {
	return c.commands[len(c.commands)-1].PipeTo(command)
}

func (c *pipedCommands) stdErrEventHandler() {
	defer c.wg.Done()
	for {
		select {
		case e := <-c.sink:
			c.stderr.Write([]byte(e.command + ": "))
			c.stderr.Write(e.data)
		}
	}
}

func (c *pipedCommands) Start() (err error) {
	defer func() {
		if err != nil {
			for _, cmd := range c.commands {
				if cmd.Running() {
					_ = cmd.Signal(syscall.SIGKILL)
				}
			}
			close(c.sink)
		}
	}()
	if c.log != nil {
		c.log.Info(c.String())
	}
	c.sink = make(chan subcommandStdErrEvent, 100*len(c.commands))
	c.wg.Add(1)
	go c.stdErrEventHandler()
	for _, cmd := range c.commands {
		stdErrWriter := &subcommandStdErr{
			command: cmd.String(),
		}
		cmd.SetStderr(stdErrWriter)
		err = cmd.Start()
		if err != nil {
			if c.log != nil {
				c.log.Error(fmt.Sprintf("Failed to start command [%s]: %v", cmd, err))
			}
			return
		}
	}

	return nil
}

func (c *pipedCommands) Wait() (err error) {
	defer func() {
		if err != nil {
			for _, cmd := range c.commands {
				if cmd.Running() {
					_ = cmd.Signal(syscall.SIGKILL)
				}
			}
		}
		close(c.sink)
		c.wg.Wait()
	}()
	for _, cmd := range c.commands {
		err = cmd.Wait()
		if err != nil {
			if c.log != nil {
				c.log.Error(fmt.Sprintf("Failed to wait for command [%s]: %v", cmd, err))
			}
			return
		}
	}
	return nil
}

func (c *pipedCommands) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *pipedCommands) Signal(sig syscall.Signal) error {
	for _, cmd := range c.commands {
		if err := cmd.Signal(sig); err != nil {
			return err
		}
	}
	return nil
}
