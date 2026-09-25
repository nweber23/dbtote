package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	logLevel  string
	logFormat string
	noColor   bool
)

// NewRootCommand creates the root command for the CLI application.
// It is a constructor so tests can create isolted instances without global flags
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "dbtote",
		Short:         "Backup and restore databases with compression, encryption, and cloud storage support",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "Path to config file (default: $XDG_CONFIG_HOME/dbtote/config.yaml)")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "debug|info|warn|error")
	root.PersistentFlags().StringVar(&logFormat, "log-format", "text", "text|json")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	var showVersion bool
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version and exit")
	originalRunE := root.RunE
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Fprintf(cmd.OutOrStdout(), "dbtote %s (commit %s, built %s, %s)\n",
				Version, Commit, BuildDate, goVersion())
			return nil
		}
		if originalRunE != nil {
			return originalRunE(cmd, args)
		}
		return cmd.Help()
	}
	root.AddCommand(newVersionCommand())
	root.AddCommand(newBackupCommand())
	root.AddCommand(newRestoreCommand())
	root.AddCommand(newSecretCommand())
	root.AddCommand(newConfigCommand())
	root.AddCommand(newTestConnectionCommand())
	root.AddCommand(newListCommand())
	root.AddCommand(newRetentionCommand())
	return root
}

// Execute runs the root command against os.Args and returns the process
func Execute() int {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		var ee exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		return 1
	}
	return 0
}
