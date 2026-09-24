package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_RedactsKnownSecretKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := New("json", slog.LevelInfo, &buf)
	logger.Info("backup completed", "target", "prod-mysql", "password", "super-secret", "status", "success")

	out := buf.String()
	if strings.Contains(out, "super-secret") {
		t.Errorf("expected password value to be redacted, got: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker, got: %s", out)
	}
	if !strings.Contains(out, "prod-mysql") {
		t.Errorf("expected non-secret fields to pass through, got: %s", out)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseLevel("bogus"); err == nil {
		t.Error("expected an error for an unknown level")
	}
}
