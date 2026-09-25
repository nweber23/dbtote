package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestListCommand_ListsBackupsInLocalStorage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prod-mysql_full_2026-09-22T02-00-00Z.sql.gz"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write backup file: %v", err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
version: 1
defaults:
  storage: local-main
storage:
  local-main:
    type: local
    path: `+dir+`
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"list", "--config", cfgPath, "--state-db", filepath.Join(dir, "state.db")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("prod-mysql")) {
		t.Errorf("expected output to mention prod-mysql, got: %s", buf.String())
	}
}
