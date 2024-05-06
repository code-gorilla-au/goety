package logging

import (
	"log/slog"
	"os"
)

var baseLogger *slog.Logger

func init() {
	baseLogger = logger(slog.LevelInfo).With(ParamApp, AppName)
}

func New(verbose bool) *slog.Logger {
	if baseLogger == nil {
		// should not occur
		baseLogger = slog.Default()
	}

	if verbose {
		baseLogger = logger(slog.LevelDebug)
	}

	return baseLogger
}

func logger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}))
}
