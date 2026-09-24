package cli

import "github.com/spf13/cobra"

// mustMarkFlagRequired marks name required on cmd. It only errors when name
// isn't a registered flag on cmd, which is a programming mistake caught the
// moment the CLI starts, not a runtime condition callers need to handle.
func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}

// exitError carries the specific process exit code a command failure
// should produce, rather than
// every failure collapsing to exit code 1.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }
func (e exitError) Unwrap() error { return e.err }
