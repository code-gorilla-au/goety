package logging

import (
	"context"
	"log/slog"
)

type loggerContextKey struct {
}

// FromContext - fetches the logger from the request context.
// If no logger is found, returns the base logger.
func FromContext(ctx context.Context) *slog.Logger {
	maybeLogger := ctx.Value(loggerContextKey{})

	if existingLogger, ok := maybeLogger.(*slog.Logger); ok {
		return existingLogger
	}

	return baseLogger
}

// WithContext - attaches logger to context
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}
