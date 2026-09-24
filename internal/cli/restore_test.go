package cli

import (
	"bytes"
	"testing"
)

func TestRestoreCommand_RequiresYesFlag(t *testing.T) {
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"restore", "--target", "t", "--from", "backup.sql", "--host", "h", "--user", "u", "--database", "d"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error when --yes is omitted for a destructive restore")
	}
}
