package cmdr

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"syscall"
)

type simpleCommand struct {
	command   *exec.Cmd
	log       *slog.Logger
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	exitError *exec.ExitError
	exitCode  int
}

func (c *simpleCommand) Signal(sig syscall.Signal) error {
	if c.command.Process == nil {
		return errors.New("Process not started")
	}
	return c.command.Process.Signal(sig)
}

func NewCommand(command string, args ...string) Command {
	return &simpleCommand{
		command: exec.Command(command, args...),
		stdin:   nil,
		stdout:  nil,
		stderr:  nil,
	}
}

func (c *simpleCommand) String() string {
	return c.command.String()
}

func (c *simpleCommand) SetLogger(logger *slog.Logger) {
	c.log = logger
}

func (c *simpleCommand) GetExitCode() int {
	return c.exitCode
}

func (c *simpleCommand) GetExitError() *exec.ExitError {
	return c.exitError
}

func (c *simpleCommand) GetStdoutAsString() string {
	if bb, ok := c.stdout.(*bytes.Buffer); ok {
		return bb.String()
	}
	return ""
}

func (c *simpleCommand) GetStderrAsString() string {
	if bb, ok := c.stderr.(*bytes.Buffer); ok {
		return bb.String()
	}
	return ""
}

func (c *simpleCommand) SetStdin(stdin io.Reader) {
	c.stdin = stdin
}

func (c *simpleCommand) SetStdout(stdout io.Writer) {
	c.stdout = stdout
}

func (c *simpleCommand) SetStderr(stderr io.Writer) {
	c.stderr = stderr
}

func (c *simpleCommand) SetUnifiedOutput(sink func(UnifiedCommandOuput) error) {
	c.stdout = &unifiedOutputSink{
		typ:  Stdout,
		sink: sink,
	}
	c.stderr = &unifiedOutputSink{
		typ:  Stderr,
		sink: sink,
	}
}

func (c *simpleCommand) SetInteractiveSink(interactiveSink InteractiveSink) {
	c.stdin = interactiveSink.Responder()
	c.stdout = &unifiedOutputSink{
		typ:  Stdout,
		sink: interactiveSink.Sink,
	}
	c.stderr = &unifiedOutputSink{
		typ:  Stderr,
		sink: interactiveSink.Sink,
	}
}

func (c *simpleCommand) PipeTo(command Command) error {
	pipe, err := c.command.StdoutPipe()
	if err != nil {
		return err
	}
	command.SetStdin(pipe)
	return nil
}

func (c *simpleCommand) Running() bool {
	return c.command.Process != nil
}

func (c *simpleCommand) Start() error {
	if c.stdin != nil && c.command.Stdin == nil {
		c.command.Stdin = c.stdin
	}
	if c.command.Stdout == nil {
		if c.stdout == nil {
			c.stdout = new(bytes.Buffer)
		}
		c.command.Stdout = c.stdout
	}
	if c.command.Stderr == nil {
		if c.stderr == nil {
			c.stderr = new(bytes.Buffer)
		}
		c.command.Stderr = c.stderr
	}
	if c.log != nil {
		c.log.Info(c.command.String())
	}

	err := c.command.Start()
	if err != nil {
		if c.log != nil {
			c.log.Error(fmt.Sprintf("Error starting [%s]: %s", c.command.String(), err))
		}
		return err
	}
	return nil
}

func (c *simpleCommand) Wait() error {
	if err := c.command.Wait(); err != nil {
		if c.log != nil {
			c.log.Error(fmt.Sprintf("Error waiting for [%s]: %s", c.command.String(), err))
		}
		if errors.As(err, &c.exitError) {
			c.exitCode = c.exitError.ExitCode()
		}
		return err
	}
	return nil
}

func (c *simpleCommand) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}
