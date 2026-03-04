package natsx

import (
	"log/slog"
	"os"
)

// Logger defines a minimal structured logging interface.
// It is compatible with slog, zap, zerolog, and similar loggers.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// ---------- default logger (slog) ----------

type slogAdapter struct {
	l *slog.Logger
}

func newDefaultLogger() Logger {
	return &slogAdapter{
		l: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (s *slogAdapter) Debug(msg string, kv ...any) { s.l.Debug(msg, kv...) }
func (s *slogAdapter) Info(msg string, kv ...any)  { s.l.Info(msg, kv...) }
func (s *slogAdapter) Warn(msg string, kv ...any)  { s.l.Warn(msg, kv...) }
func (s *slogAdapter) Error(msg string, kv ...any) { s.l.Error(msg, kv...) }

// NopLogger is a logger that discards all output. Useful for tests.
type NopLogger struct{}

func (NopLogger) Debug(string, ...any) {}
func (NopLogger) Info(string, ...any)  {}
func (NopLogger) Warn(string, ...any)  {}
func (NopLogger) Error(string, ...any) {}
