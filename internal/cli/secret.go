package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/secrets"
)

func newSecretCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Manage credentials in the OS keyring"}
	cmd.AddCommand(newSecretSetCommand(), newSecretGetCommand(), newSecretRmCommand())
	return cmd
}

func newSecretSetCommand() *cobra.Command {
	var target string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Prompt for a password and store it in the OS keyring",
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := promptForPassword()
			if err != nil {
				return exitError{code: 2, err: err}
			}
			if err := secrets.NewKeyringStore().Set(cmd.Context(), target, password); err != nil {
				return exitError{code: 1, err: err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stored credential for target %q\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target name (required)")
	mustMarkFlagRequired(cmd, "target")
	return cmd
}

func newSecretGetCommand() *cobra.Command {
	var target string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Print a stored credential (debugging only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return exitError{code: 2, err: fmt.Errorf("cli: --yes-i-know is required to print a secret to stdout")}
			}
			v, err := secrets.NewKeyringStore().Get(cmd.Context(), target)
			if err != nil {
				return exitError{code: 1, err: err}
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target name (required)")
	cmd.Flags().BoolVar(&confirm, "yes-i-know", false, "Required to print a secret to stdout")
	mustMarkFlagRequired(cmd, "target")
	return cmd
}

func newSecretRmCommand() *cobra.Command {
	var target string
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a stored credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := secrets.NewKeyringStore().Delete(cmd.Context(), target); err != nil {
				return exitError{code: 1, err: err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed credential for target %q\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target name (required)")
	mustMarkFlagRequired(cmd, "target")
	return cmd
}
