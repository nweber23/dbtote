package backupjob

import (
	"testing"
	"time"
)

func TestParseFilename_RoundTripsWithBuildFilename(t *testing.T) {
	ts := time.Date(2026, 9, 22, 2, 0, 0, 0, time.UTC)
	name := BuildFilename("prod-mysql", "full", ts, ".sql.gz.age")

	target, backupType, gotTS, ok := ParseFilename(name)
	if !ok {
		t.Fatalf("ParseFilename(%q) returned ok=false", name)
	}
	if target != "prod-mysql" || backupType != "full" {
		t.Errorf("target=%q backupType=%q, want prod-mysql/full", target, backupType)
	}
	if !gotTS.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", gotTS, ts)
	}
}

func TestParseFilename_RejectsUnrecognizedNames(t *testing.T) {
	if _, _, _, ok := ParseFilename("not-a-dbtote-filename.txt"); ok {
		t.Error("expected ok=false for a non-conforming filename")
	}
}
