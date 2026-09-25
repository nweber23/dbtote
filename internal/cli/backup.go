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
	"github.com/nweber23/dbtote/internal/config"
	"github.com/nweber23/dbtote/internal/logging"
	"github.com/nweber23/dbtote/internal/notify/slack"
	"github.com/nweber23/dbtote/internal/retention"
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
		notifyFlag  bool
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

			rt := resolveTarget(cmd, target, host, user, database, port, passwordEnv, output, recipient)
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

			if encrypt && len(rt.Recipients) == 0 {
				return exitError{code: 2, err: fmt.Errorf("cli: --encrypt requires --recipient or encryption.recipients in config")}
			}

			dumpExt := "sql"
			if rt.Engine == "postgres" {
				dumpExt = "dump"
			}

			var stateDB *backupjob.StateDB
			var retentionPolicy *retention.Policy
			var notifier backupjob.Notifier
			var notifyOn []string

			if path, pathErr := resolveConfigPath(); pathErr == nil {
				if cfg, loadErr := config.Load(path); loadErr == nil {
					statePath, spErr := config.DefaultStateDBPath()
					if spErr == nil {
						if db, openErr := backupjob.OpenStateDB(statePath); openErr == nil {
							stateDB = db
							defer db.Close()
						}
					}
					policy := resolveRetentionPolicy(cfg, target)
					if policy.KeepLast != 0 || policy.KeepDays != 0 {
						retentionPolicy = &policy
					}
					if cfg.Notify.Slack.WebhookURLEnv != "" {
						if url := os.Getenv(cfg.Notify.Slack.WebhookURLEnv); url != "" {
							notifier = slack.New(url)
							notifyOn = cfg.Notify.Slack.On
						}
					}
				}
			}

			notifyExplicitlySet := cmd.Flags().Changed("notify") || cmd.Flags().Changed("no-notify")
			if notifyExplicitlySet && !notifyFlag {
				notifier = nil
			}

			result, err := backupjob.Run(ctx, backupjob.Options{
				TargetName:      target,
				Engine:          rt.Engine,
				Connection:      rt.Connection,
				DumpFileExt:     dumpExt,
				Compress:        compressAlg,
				Encrypt:         encrypt,
				Recipients:      rt.Recipients,
				Storage:         local.New(rt.StoragePath),
				StorageName:     rt.StorageName,
				State:           stateDB,
				RetentionPolicy: retentionPolicy,
				Notifier:        notifier,
				NotifyOn:        notifyOn,
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
	cmd.Flags().BoolVar(&notifyFlag, "notify", true, "Send a notification for this run (overrides config); pass --notify=false as the --no-notify equivalent")
	cmd.Flags().Lookup("notify").NoOptDefVal = "true"
	mustMarkFlagRequired(cmd, "target")

	return cmd
}
