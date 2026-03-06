package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var Log zerolog.Logger

func Init(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var w io.Writer
	if os.Getenv("LOG_FORMAT") == "json" {
		w = os.Stdout
	} else {
		w = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	Log = zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Logger()
}

func With(ctx string) zerolog.Logger {
	return Log.With().Str("context", ctx).Logger()
}
