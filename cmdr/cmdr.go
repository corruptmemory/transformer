package cmdr

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
)

type OutputTag int

const (
	Stdout OutputTag = iota
	Stderr
)

type UnifiedCommandOuput struct {
	Type OutputTag
	Data []byte
}

type unifiedOutputSink struct {
	typ  OutputTag
	sink func(UnifiedCommandOuput) error
}

func (u unifiedOutputSink) Write(p []byte) (n int, err error) {
	err = u.sink(UnifiedCommandOuput{
		Type: u.typ,
		Data: p,
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

type InteractiveSink interface {
	Responder() io.Reader
	Sink(output UnifiedCommandOuput) error
}

type Command struct {
	command   *exec.Cmd
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	exitError *exec.ExitError
	exitCode  int
}

func NewCommand(command string, args ...string) *Command {
	return &Command{
		command: exec.Command(command, args...),
		stdin:   nil,
		stdout:  nil,
		stderr:  nil,
	}
}

func (c *Command) GetExitCode() int {
	return c.exitCode
}

func (c *Command) GetExitError() *exec.ExitError {
	return c.exitError
}

func (c *Command) GetStdout() io.Writer {
	return c.stdout
}

func (c *Command) GetStdoutAsString() string {
	if bb, ok := c.stdout.(*bytes.Buffer); ok {
		return bb.String()
	}
	return ""
}

func (c *Command) GetStderr() io.Writer {
	return c.stderr
}

func (c *Command) GetStderrAsString() string {
	if bb, ok := c.stderr.(*bytes.Buffer); ok {
		return bb.String()
	}
	return ""
}

func (c *Command) SetStdin(stdin io.Reader) {
	c.stdin = stdin
}

func (c *Command) SetStdout(stdout io.Writer) {
	c.stdout = stdout
}

func (c *Command) SetStderr(stderr io.Writer) {
	c.stderr = stderr
}

func (c *Command) SetUnifiedOutput(sink func(UnifiedCommandOuput) error) {
	c.stdout = &unifiedOutputSink{
		typ:  Stdout,
		sink: sink,
	}
	c.stderr = &unifiedOutputSink{
		typ:  Stderr,
		sink: sink,
	}
}

func (c *Command) SetUnifiedOutputWitBuilder(sinkBuilder func(cmd *exec.Cmd) func(UnifiedCommandOuput) error) {
	sink := sinkBuilder(c.command)
	c.stdout = &unifiedOutputSink{
		typ:  Stdout,
		sink: sink,
	}
	c.stderr = &unifiedOutputSink{
		typ:  Stderr,
		sink: sink,
	}
}

func (c *Command) SetInteractiveSink(interactiveSink InteractiveSink) {
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

func (c *Command) SetInteractiveSinkWithBuilder(interactiveSinkBuilder func(cmd *exec.Cmd) InteractiveSink) {
	interactiveSink := interactiveSinkBuilder(c.command)
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

func (c *Command) PipeTo(command *Command) error {
	pipe, err := c.command.StdoutPipe()
	if err != nil {
		return err
	}
	command.command.Stdin = pipe
	return nil
}

func (c *Command) Start() error {
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

	return c.command.Start()
}

func (c *Command) Wait() error {
	err := c.command.Wait()
	if err != nil {
		if errors.As(err, &c.exitError) {
			c.exitCode = c.exitError.ExitCode()
		}
	}
	return err
}

func (c *Command) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}
