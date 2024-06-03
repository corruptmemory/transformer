package capturelogger

import (
	"bytes"
	"log/slog"
	"sync"
)

type CaptureLogger struct {
	wg        sync.WaitGroup
	logBuffer *bytes.Buffer
	logger    *slog.Logger
	logQueue  chan []byte
}

func (c *CaptureLogger) Close() error {
	close(c.logQueue)
	c.wg.Wait()
	return nil
}

func (c *CaptureLogger) Write(p []byte) (n int, err error) {
	c.logBuffer.Write(p)
	return len(p), nil
}

func NewCaptureLogger(queueLength int, options *slog.HandlerOptions) *CaptureLogger {
	var r *CaptureLogger
	th := slog.NewTextHandler(r, options)
	r = &CaptureLogger{
		logBuffer: new(bytes.Buffer),
		logger:    slog.New(th),
		logQueue:  make(chan []byte, queueLength),
	}
	r.wg.Add(1)
	go r.logEventHandler()
	return r
}

func NewDefaultCaptureLogger() *CaptureLogger {
	return NewCaptureLogger(1000, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})
}

func (c *CaptureLogger) logEventHandler() {
	defer c.wg.Done()
	for {
		select {
		case e := <-c.logQueue:
			c.logBuffer.Write(e)
		}
	}
}

func (c *CaptureLogger) Log() *slog.Logger {
	return c.logger
}

func (c *CaptureLogger) LogBuffer() *bytes.Buffer {
	return c.logBuffer
}

func WithCaptureLoggerLogger[T any](l *CaptureLogger, f func(l *slog.Logger) (T, error)) (T, error) {
	return f(l.logger)
}

func CaptureLoggerLoggerSessionBuilder[T any](queueLength int, options *slog.HandlerOptions) func(body func(l *slog.Logger) (T, error)) (*CaptureLogger, T, error) {
	return func(body func(l *slog.Logger) (T, error)) (*CaptureLogger, T, error) {
		l := NewCaptureLogger(queueLength, options)
		result, err := body(l.Log())
		l.Close()
		return l, result, err
	}
}
