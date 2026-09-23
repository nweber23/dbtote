package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit = "none"
	BuildDate = "unknown"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit hash, build date, and Go version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "dbtote %s (commit %s, built %s, %s)\n",
				Version, Commit, BuildDate, goVersion())
			return nil
		},
	}
}

func goVersion() string {
	return runtime.Version()
}