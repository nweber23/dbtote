package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

var redactedKeys = map[string]struct{}{
	"password":          {},
	"pass":              {},
	"secret":            {},
	"token":             {},
	"recipient":         {},
	"identity":          {},
	"uri":               {},
	"connection_string": {},
	"dsn":               {},
}

type redactingHandler struct {
	next slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	redacted := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		redacted.AddAttrs(redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func redactAttr(a slog.Attr) slog.Attr {
	if _, sensitive := redactedKeys[strings.ToLower(a.Key)]; sensitive {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &redactingHandler{next: h.next.WithAttrs(attrs)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

// New builds a logger in "text" or "json" format, wrapped in redaction.
func New(format string, level slog.Level, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	var base slog.Handler
	if format == "json" {
		base = slog.NewJSONHandler(w, opts)
	} else {
		base = slog.NewTextHandler(w, opts)
	}
	return slog.New(&redactingHandler{next: base})
}

// ParseLevel converts the CLI's --log-level string into a slog.Level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: unknown level %q", s)
	}
}
