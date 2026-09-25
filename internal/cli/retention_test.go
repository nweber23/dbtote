package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nweber23/dbtote/internal/backupjob"
)

func TestRetentionPreview_ListsWhatWouldBePruned(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	db, err := backupjob.OpenStateDB(statePath)
	if err != nil {
		t.Fatalf("OpenStateDB: %v", err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	if err := db.RecordBackup(context.Background(), backupjob.BackupRecord{Filename: "old.sql.gz", Target: "t", Timestamp: old}); err != nil {
		t.Fatalf("RecordBackup: %v", err)
	}
	db.Close()

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
version: 1
targets:
  t:
    engine: mysql
    retention:
      keep_last: 0
      keep_days: 1
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"retention", "preview", "--target", "t", "--config", cfgPath, "--state-db", statePath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("old.sql.gz")) {
		t.Errorf("expected preview to mention old.sql.gz, got: %s", buf.String())
	}
}
