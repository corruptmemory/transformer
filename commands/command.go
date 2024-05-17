package commands

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"transformer/cmdr"

	"github.com/mitchellh/go-homedir"
)

type Commands struct {
	log       *slog.Logger
	needsSudo bool
	homeDir   string
	tempRoot  string
}

func InMemoryHandler() (slog.Handler, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
	return handler, buf
}

func InMemoryCommands(needsSudo bool) (*Commands, *bytes.Buffer, error) {
	handler, buf := InMemoryHandler()
	log := slog.New(handler)
	c, err := NewCommands(log, needsSudo, "")
	if err != nil {
		return nil, nil, err
	}
	return c, buf, nil
}

func DefaultCommands(needsSudo bool) (*Commands, error) {
	return NewCommands(slog.Default(), needsSudo, "")
}

func NewCommands(log *slog.Logger, needsSudo bool, tempRoot string) (*Commands, error) {
	hd, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	return &Commands{
		log:       log,
		needsSudo: needsSudo,
		homeDir:   hd,
		tempRoot:  tempRoot,
	}, nil
}

func (c *Commands) With(args ...any) *Commands {
	return &Commands{
		needsSudo: c.needsSudo,
		log:       c.log.With(args...),
		homeDir:   c.homeDir,
		tempRoot:  c.tempRoot,
	}
}

func (c *Commands) WithTempRoot(tempRoot string) *Commands {
	return &Commands{
		needsSudo: c.needsSudo,
		log:       c.log,
		homeDir:   c.homeDir,
		tempRoot:  tempRoot,
	}
}

func (c *Commands) TempDir(pattern string) (string, error) {
	return os.MkdirTemp(c.tempRoot, pattern)
}

func (c *Commands) ExpandPath(path string) (out string, err error) {
	exp, err := homedir.Expand(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(exp)
}

func (c *Commands) MkdirAll(path string, perm os.FileMode) (out string, err error) {
	ep, err := c.ExpandPath(path)
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(ep, perm)
	if err != nil {
		return "", err
	}
	c.Debugf("created directory %s", ep)
	return ep, nil
}

func (c *Commands) Cwd() (string, error) {
	return os.Getwd()
}

func (c *Commands) HomeDir() string {
	return c.homeDir
}

func (c *Commands) WithGroup(name string) *Commands {
	return &Commands{
		needsSudo: c.needsSudo,
		log:       c.log.WithGroup(name),
		homeDir:   c.homeDir,
		tempRoot:  c.tempRoot,
	}
}

func WithGroupInScopeReturning[T any](c *Commands, name string, body func(c1 *Commands) T) T {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
		homeDir:   c.homeDir,
		tempRoot:  c.tempRoot,
	}
	return body(c1)
}

func WithGroupInScopeReturningErr[T any](c *Commands, name string, body func(c1 *Commands) (T, error)) (T, error) {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
		homeDir:   c.homeDir,
		tempRoot:  c.tempRoot,
	}
	return body(c1)
}

func (c *Commands) WithGroupInScopeErr(name string, body func(c1 *Commands) error) error {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
		homeDir:   c.homeDir,
		tempRoot:  c.tempRoot,
	}
	return body(c1)
}

func (c *Commands) WithGroupInScope(name string, body func(c1 *Commands)) {
	log := c.log.WithGroup(name)
	c1 := &Commands{
		needsSudo: c.needsSudo,
		log:       log,
		homeDir:   c.homeDir,
		tempRoot:  c.tempRoot,
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
