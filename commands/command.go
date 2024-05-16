package commands

import (
	"bytes"
	"fmt"
	"log/slog"

	"transformer/cmdr"
)

type Commands struct {
	log       *slog.Logger
	needsSudo bool
}

func InMemoryHandler() (slog.Handler, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
	return handler, buf
}

func InMemoryCommands(needsSudo bool) (*Commands, *bytes.Buffer) {
	handler, buf := InMemoryHandler()
	log := slog.New(handler)
	return NewCommands(log, needsSudo), buf
}

func DefaultCommands(needsSudo bool) *Commands {
	return NewCommands(slog.Default(), needsSudo)
}

func NewCommands(log *slog.Logger, needsSudo bool) *Commands {
	return &Commands{
		log:       log,
		needsSudo: needsSudo,
	}
}

func (c *Commands) With(args ...any) *Commands {
	return &Commands{
		needsSudo: c.needsSudo,
		log:       c.log.With(args...),
	}
}

func (c *Commands) WithGroup(name string) *Commands {
	return &Commands{
		needsSudo: c.needsSudo,
		log:       c.log.WithGroup(name),
	}
}

func WithGroupInScopeReturning[T any](c *Commands, name string, body func(c1 *Commands) T) T {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
	}
	return body(c1)
}

func WithGroupInScopeReturningErr[T any](c *Commands, name string, body func(c1 *Commands) (T, error)) (T, error) {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
	}
	return body(c1)
}

func (c *Commands) WithGroupInScopeErr(name string, body func(c1 *Commands) error) error {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
	}
	return body(c1)
}

func (c *Commands) WithGroupInScope(name string, body func(c1 *Commands)) {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
	}
	body(c1)
}

func (c *Commands) Debug(msg string, args ...any) {
	c.log.Debug(msg, args...)
}

func (c *Commands) Debugf(format string, args ...any) {
	c.log.Debug(fmt.Sprintf(format, args...))
}

func (c *Commands) Info(msg string, args ...any) {
	c.log.Info(msg, args...)
}

func (c *Commands) Infof(format string, args ...any) {
	c.log.Info(fmt.Sprintf(format, args...))
}

func (c *Commands) Warn(msg string, args ...any) {
	c.log.Warn(msg, args...)
}

func (c *Commands) Warnf(format string, args ...any) {
	c.log.Warn(fmt.Sprintf(format, args...))
}

func (c *Commands) Error(msg string, args ...any) {
	c.log.Error(msg, args...)
}

func (c *Commands) Errorf(format string, args ...any) {
	c.log.Error(fmt.Sprintf(format, args...))
}

func (c *Commands) Cmd(command string, args ...string) cmdr.Command {
	var cmd string
	var newArgs []string
	if c.needsSudo {
		cmd = "sudo"
		newArgs = []string{command}
	} else {
		cmd = command
		newArgs = []string{}
	}
	newArgs = append(newArgs, args...)

	nc := cmdr.NewCommand(cmd, newArgs...)
	nc.SetLogger(c.log)
	return nc
}

func (c *Commands) CmdWithGroup(group, command string, args ...string) cmdr.Command {
	var cmd string
	var newArgs []string
	if c.needsSudo {
		cmd = "sudo"
		newArgs = []string{command}
	} else {
		cmd = command
		newArgs = []string{}
	}
	newArgs = append(newArgs, args...)

	nc := cmdr.NewCommand(cmd, newArgs...)
	nc.SetLogger(c.log.WithGroup(group))
	return nc
}

func (c *Commands) RunCmd(command cmdr.Command) error {
	err := command.Run()
	if err != nil {
		name := command.String()
		c.Errorf("%s: error = %v", name, err)
		c.Errorf(" > exit code: %d", command.GetExitCode())
		c.Errorf(" > exit error: %+v", command.GetExitError())
		c.Errorf(" > stdout: %s", command.GetStdoutAsString())
		c.Errorf(" > stderr: %s", command.GetStderrAsString())
		return fmt.Errorf("%s: error = %v", name, err)
	}
	return nil
}
