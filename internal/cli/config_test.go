package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidate_FailsOnBrokenConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\ndefaults:\n  storage: missing\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"config", "validate", "--config", path})

	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for a config referencing an undefined storage backend")
	}
}

func TestConfigInit_WritesAStarterFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetIn(bytes.NewBufferString("/var/backups/dbtote\n"))
	root.SetArgs([]string{"config", "init", "--config", path})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be written: %v", err)
	}
}
