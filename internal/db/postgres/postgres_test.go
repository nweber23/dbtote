package postgres

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nweber23/dbtote/internal/driver"
)

func writeFakeBinary(t *testing.T, name, stdout, stderr string, exitCode int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script binaries are POSIX-shell only")
	}
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' %q\nprintf '%%s' %q >&2\nexit %d\n", stdout, stderr, exitCode)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBackup_StreamsDumpOutput(t *testing.T) {
	writeFakeBinary(t, "pg_dump", "-- fake custom-format dump --", "", 0)

	_, backuper, _ := New(driver.ConnectionConfig{Host: "db", Port: 5432, User: "u", Password: "p", Database: "d"})
	var out bytes.Buffer
	result, err := backuper.Backup(context.Background(), driver.BackupOptions{Database: "d", Output: &out})
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if out.String() != "-- fake custom-format dump --" {
		t.Errorf("got %q", out.String())
	}
	if result.BytesWritten != int64(len(out.String())) {
		t.Errorf("BytesWritten = %d, want %d", result.BytesWritten, len(out.String()))
	}
}

func TestBackup_WrapsCommandFailure(t *testing.T) {
	writeFakeBinary(t, "pg_dump", "", "connection refused", 1)

	_, backuper, _ := New(driver.ConnectionConfig{Host: "db", Port: 5432, User: "u", Password: "p", Database: "d"})
	_, err := backuper.Backup(context.Background(), driver.BackupOptions{Database: "d", Output: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestRestore_FeedsInputToStdin(t *testing.T) {
	writeFakeBinary(t, "pg_restore", "", "", 0)

	_, _, restorer := New(driver.ConnectionConfig{Host: "db", Port: 5432, User: "u", Password: "p", Database: "d"})
	err := restorer.Restore(context.Background(), driver.RestoreOptions{Database: "d", Input: bytes.NewBufferString("custom-format-bytes")})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
}

func TestConnector_PingBeforeConnectErrors(t *testing.T) {
	connector, _, _ := New(driver.ConnectionConfig{})
	if err := connector.Ping(context.Background()); err == nil {
		t.Error("expected Ping before Connect to error")
	}
}

func TestCapabilities(t *testing.T) {
	_, backuper, restorer := New(driver.ConnectionConfig{})
	if !backuper.SupportsIncremental() {
		t.Error("expected SupportsIncremental() to be true")
	}
	if backuper.IncrementalBasis() != driver.BasisWAL {
		t.Errorf("IncrementalBasis() = %v, want BasisWAL", backuper.IncrementalBasis())
	}
	if !restorer.SupportsSelective() {
		t.Error("expected SupportsSelective() to be true")
	}
}
