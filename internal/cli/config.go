package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/config"
	"github.com/nweber23/dbtote/internal/secrets"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Validate or initialize the dbtote config file"}
	cmd.AddCommand(newConfigValidateCommand(), newConfigInitCommand())
	return cmd
}

func resolveConfigPath() (string, error) {
	if cfgFile != "" {
		return cfgFile, nil
	}
	return config.DefaultPath()
}

func newConfigValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate schema, storage references, age keys, cron expressions, and credential resolvability",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return exitError{code: 2, err: err}
			}
			cfg, err := config.Load(path)
			if err != nil {
				return exitError{code: 2, err: err}
			}
			if err := cfg.Validate(); err != nil {
				return exitError{code: 2, err: err}
			}
			if err := cfg.ValidateCredentials(cmd.Context(), secrets.NewKeyringStore()); err != nil {
				return exitError{code: 2, err: err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config %q is valid\n", path)
			return nil
		},
	}
}

func newConfigInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write a starter config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return exitError{code: 2, err: err}
			}

			fmt.Fprint(cmd.OutOrStdout(), "Local backup storage path [/var/backups/dbtote]: ")
			line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			storagePath := strings.TrimSpace(line)
			if storagePath == "" {
				storagePath = "/var/backups/dbtote"
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return exitError{code: 1, err: fmt.Errorf("cli: create config directory: %w", err)}
			}
			contents := fmt.Sprintf(starterTemplate, storagePath)
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				return exitError{code: 1, err: fmt.Errorf("cli: write config: %w", err)}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			return nil
		},
	}
}

const starterTemplate = `version: 1

defaults:
  compress: gzip
  encrypt: false
  storage: local-main
  retention:
    keep_last: 7
    keep_days: 30

storage:
  local-main:
    type: local
    path: %s

# Add targets here, e.g.:
# targets:
#   prod-mysql:
#     engine: mysql
#     host: db1.internal
#     port: 3306
#     user: backup_svc
#     database: app_production
#     storage: local-main
targets: {}
`
