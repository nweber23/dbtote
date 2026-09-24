package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/backupjob"
	"github.com/nweber23/dbtote/internal/driver"
	"github.com/nweber23/dbtote/internal/logging"
	"github.com/nweber23/dbtote/internal/secrets"
	"github.com/nweber23/dbtote/internal/storage/local"
)

func promptForPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("cli: read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func newBackupCommand() *cobra.Command {
	var (
		target      string
		host        string
		port        int
		user        string
		database    string
		passwordEnv string
		compressAlg string
		encrypt     bool
		recipient   string
		output      string
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Run a single backup job",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			level, err := logging.ParseLevel(logLevel)
			if err != nil {
				return exitError{code: 2, err: err}
			}
			logger := logging.New(logFormat, level, cmd.OutOrStdout())

			password, err := secrets.ResolvePassword(ctx, secrets.NewKeyringStore(), secrets.ResolveOptions{
				EnvVarName: passwordEnv,
				KeyringKey: target,
				Prompt:     promptForPassword,
			})
			if err != nil {
				return exitError{code: 2, err: err}
			}

			var recipients []string
			if recipient != "" {
				recipients = []string{recipient}
			}
			if encrypt && len(recipients) == 0 {
				return exitError{code: 2, err: fmt.Errorf("cli: --encrypt requires --recipient")}
			}

			result, err := backupjob.Run(ctx, backupjob.Options{
				TargetName: target,
				Engine:     "mysql",
				Connection: driver.ConnectionConfig{
					Host: host, Port: port, User: user, Password: password, Database: database,
				},
				DumpFileExt: "sql",
				Compress:    compressAlg,
				Encrypt:     encrypt,
				Recipients:  recipients,
				Storage:     local.New(output),
			}, logger)
			if err != nil {
				var be *backupjob.Error
				if errors.As(err, &be) && (be.Stage == "connect" || be.Stage == "test-connection") {
					return exitError{code: 3, err: err}
				}
				return exitError{code: 1, err: err}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "backup complete: %s (%d bytes, %s)\n", result.Filename, result.BytesWritten, result.Duration)
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Name of the backup target (required)")
	cmd.Flags().StringVar(&host, "host", "", "Database host")
	cmd.Flags().IntVar(&port, "port", 3306, "Database port")
	cmd.Flags().StringVar(&user, "user", "", "Database user")
	cmd.Flags().StringVar(&database, "database", "", "Database name")
	cmd.Flags().StringVar(&passwordEnv, "password-env", "", "Env var to read the password from instead of the keyring")
	cmd.Flags().StringVar(&compressAlg, "compress", "gzip", "none|gzip")
	cmd.Flags().BoolVar(&encrypt, "encrypt", false, "Encrypt output with age")
	cmd.Flags().StringVar(&recipient, "recipient", "", "age public key to encrypt to")
	cmd.Flags().StringVar(&output, "output", ".", "Local directory to write the backup into")
	mustMarkFlagRequired(cmd, "target")
	mustMarkFlagRequired(cmd, "host")
	mustMarkFlagRequired(cmd, "user")
	mustMarkFlagRequired(cmd, "database")

	return cmd
}
