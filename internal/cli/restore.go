package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/compress"
	"github.com/nweber23/dbtote/internal/crypto"
	"github.com/nweber23/dbtote/internal/driver"
	"github.com/nweber23/dbtote/internal/secrets"
)

// unwrapPipeline reverses the backup pipeline based on filename.ext chain:
// strip and decrypt ".age" first, then strip and gunzip ".gz" — the exact
// inverse of the order backupjob.Run applies them.
func unwrapPipeline(r io.Reader, filename, decryptKeyPath string) (io.Reader, error) {
	name := filename
	if strings.HasSuffix(name, ".age") {
		identity, err := loadIdentity(decryptKeyPath)
		if err != nil {
			return nil, err
		}
		decrypted, err := crypto.NewDecryptReader(r, identity)
		if err != nil {
			return nil, fmt.Errorf("cli: decrypt: %w", err)
		}
		r = decrypted
		name = strings.TrimSuffix(name, ".age")
	}
	if strings.HasSuffix(name, ".gz") {
		gr, err := compress.NewReader("gzip", r)
		if err != nil {
			return nil, fmt.Errorf("cli: decompress: %w", err)
		}
		r = gr
	}
	return r, nil
}

func loadIdentity(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("cli: --decrypt-key is required to restore an encrypted backup")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cli: read identity file %q: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func newRestoreCommand() *cobra.Command {
	var (
		target      string
		from        string
		host        string
		port        int
		user        string
		database    string
		passwordEnv string
		decryptKey  string
		yes         bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a target from a backup file",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if !yes {
				return exitError{code: 2, err: fmt.Errorf("cli: restore is destructive; re-run with --yes to proceed")}
			}

			rt := resolveTarget(cmd, target, host, user, database, port, passwordEnv, "", "")
			if rt.Connection.Host == "" || rt.Connection.User == "" || rt.Connection.Database == "" {
				return exitError{code: 2, err: fmt.Errorf("cli: --host, --user, and --database are required unless --target resolves them from config")}
			}

			password, err := secrets.ResolvePassword(ctx, secrets.NewKeyringStore(), secrets.ResolveOptions{
				EnvVarName: rt.PasswordEnv,
				KeyringKey: target,
				Prompt:     promptForPassword,
			})
			if err != nil {
				return exitError{code: 2, err: err}
			}
			rt.Connection.Password = password

			f, err := os.Open(from)
			if err != nil {
				return exitError{code: 2, err: fmt.Errorf("cli: open backup file %q: %w", from, err)}
			}
			defer f.Close()

			reader, err := unwrapPipeline(f, from, decryptKey)
			if err != nil {
				return exitError{code: 1, err: err}
			}

			factory, ok := driver.Get(rt.Engine)
			if !ok {
				return exitError{code: 2, err: fmt.Errorf("cli: unknown engine %q", rt.Engine)}
			}
			conn, _, restorer := factory(rt.Connection)

			if err := conn.Connect(ctx, rt.Connection); err != nil {
				return exitError{code: 3, err: err}
			}
			defer conn.Close()
			if err := conn.Ping(ctx); err != nil {
				return exitError{code: 3, err: err}
			}

			if err := restorer.Restore(ctx, driver.RestoreOptions{Database: rt.Connection.Database, Input: reader}); err != nil {
				return exitError{code: 1, err: err}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "restore complete: %s\n", from)
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Target to restore into (required)")
	cmd.Flags().StringVar(&from, "from", "", "Path to the backup file (required)")
	cmd.Flags().StringVar(&host, "host", "", "Database host")
	cmd.Flags().IntVar(&port, "port", 3306, "Database port")
	cmd.Flags().StringVar(&user, "user", "", "Database user")
	cmd.Flags().StringVar(&database, "database", "", "Database name")
	cmd.Flags().StringVar(&passwordEnv, "password-env", "", "Env var to read the password from instead of the keyring")
	cmd.Flags().StringVar(&decryptKey, "decrypt-key", "", "Path to an age private key file")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the destructive-restore confirmation")
	mustMarkFlagRequired(cmd, "target")
	mustMarkFlagRequired(cmd, "from")

	return cmd
}
