package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_HelpListsGlobalFlags(t *testing.T) {
	cmd := NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	for _, flag := range []string{"--config", "--log-level", "--log-format", "--no-color", "--version"} {
		if !strings.Contains(output, flag) {
			t.Errorf("Help output does not contain flag: %s", flag)
		}
	}
}

func TestRootCommand_VersionFlagPrintsVersion(t *testing.T) {
	Version = "0.0.0-test"
	cmd := NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	if !strings.Contains(buf.String(), "0.0.0-test") {
		t.Errorf("Version output does not contain expected version: 0.0.0-test, got: %s", buf.String())
	}
}