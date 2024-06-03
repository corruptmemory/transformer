package cmdr

import (
	"io"
	"log/slog"
	"os/exec"
	"syscall"
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

type AsyncRunTag int

const (
	ARStarted AsyncRunTag = iota
	ARError
	ARCompleted
)

type AsyncRun struct {
	Tag   AsyncRunTag
	PID   int
	Error error
}

type Command interface {
	String() string
	GetExitCode() int
	GetExitError() *exec.ExitError
	GetStdoutAsString() string
	GetStderrAsString() string
	SetStdin(stdin io.Reader)
	SetStdout(stdout io.Writer)
	SetStderr(stderr io.Writer)
	SetUnifiedOutput(sink func(UnifiedCommandOuput) error)
	SetInteractiveSink(interactiveSink InteractiveSink)
	SetLogger(logger *slog.Logger)
	PipeTo(command Command) error
	Start() error
	Wait() error
	Run() error
	RunAsync() chan AsyncRun
	Signal(sig syscall.Signal) error
	Running() bool
}
