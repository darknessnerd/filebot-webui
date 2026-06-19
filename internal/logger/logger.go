package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

type Logger interface {
	Debug() *zerolog.Event
	Info() *zerolog.Event
	Warn() *zerolog.Event
	Error() *zerolog.Event
}

type zerologLogger struct {
	zl zerolog.Logger
}

func (l *zerologLogger) Debug() *zerolog.Event { return l.zl.Debug() }
func (l *zerologLogger) Info() *zerolog.Event  { return l.zl.Info() }
func (l *zerologLogger) Warn() *zerolog.Event  { return l.zl.Warn() }
func (l *zerologLogger) Error() *zerolog.Event { return l.zl.Error() }

func New(level string, pretty bool) Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var w io.Writer = os.Stdout
	if pretty {
		w = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	zl := zerolog.New(w).With().Timestamp().Logger().Level(lvl)
	return &zerologLogger{zl: zl}
}
