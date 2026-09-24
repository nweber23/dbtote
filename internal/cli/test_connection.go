package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/config"
	"github.com/nweber23/dbtote/internal/driver"
	"github.com/nweber23/dbtote/internal/secrets"
)

func newTestConnectionCommand() *cobra.Command {
	var target string
	cmd := &cobra.Command{
		Use:   "test-connection",
		Short: "Validate credentials and connectivity for a configured target",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return exitError{code: 2, err: err}
			}
			cfg, err := config.Load(path)
			if err != nil {
				return exitError{code: 2, err: err}
			}
			t, ok := cfg.Targets[target]
			if !ok {
				return exitError{code: 2, err: fmt.Errorf("cli: unknown target %q", target)}
			}

			factory, ok := driver.Get(t.Engine)
			if !ok {
				return exitError{code: 2, err: fmt.Errorf("cli: unknown engine %q", t.Engine)}
			}

			password, err := secrets.ResolvePassword(cmd.Context(), secrets.NewKeyringStore(), secrets.ResolveOptions{
				EnvVarName: t.PasswordEnv,
				KeyringKey: target,
				Prompt:     promptForPassword,
			})
			if err != nil {
				return exitError{code: 2, err: err}
			}

			connCfg := driver.ConnectionConfig{Host: t.Host, Port: t.Port, User: t.User, Password: password, Database: t.Database}
			conn, _, _ := factory(connCfg)
			if err := conn.Connect(cmd.Context(), connCfg); err != nil {
				return exitError{code: 3, err: err}
			}
			defer conn.Close()
			if err := conn.Ping(cmd.Context()); err != nil {
				return exitError{code: 3, err: err}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "target %q: connection OK\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target to test (required)")
	mustMarkFlagRequired(cmd, "target")
	return cmd
}
