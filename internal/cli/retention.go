package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/backupjob"
	"github.com/nweber23/dbtote/internal/config"
	"github.com/nweber23/dbtote/internal/retention"
	"github.com/nweber23/dbtote/internal/storage"
)

func resolveRetentionPolicy(cfg *config.Config, target string) retention.Policy {
	policy := retention.Policy{KeepLast: cfg.Defaults.Retention.KeepLast, KeepDays: cfg.Defaults.Retention.KeepDays}
	if t, ok := cfg.Targets[target]; ok {
		if t.Retention.KeepLast != 0 {
			policy.KeepLast = t.Retention.KeepLast
		}
		if t.Retention.KeepDays != 0 {
			policy.KeepDays = t.Retention.KeepDays
		}
	}
	return policy
}

func newRetentionCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "retention", Short: "Preview or enforce retention policy"}
	cmd.AddCommand(newRetentionPreviewCommand(), newRetentionApplyCommand())
	return cmd
}

func retentionRun(cmd *cobra.Command, target, stateDBPath string, apply bool) error {
	path, err := resolveConfigPath()
	if err != nil {
		return exitError{code: 2, err: err}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return exitError{code: 2, err: err}
	}

	if stateDBPath == "" {
		stateDBPath, err = config.DefaultStateDBPath()
		if err != nil {
			return exitError{code: 2, err: err}
		}
	}
	db, err := backupjob.OpenStateDB(stateDBPath)
	if err != nil {
		return exitError{code: 1, err: err}
	}
	defer db.Close()

	records, err := db.ListBackups(cmd.Context(), backupjob.ListFilter{Target: target})
	if err != nil {
		return exitError{code: 1, err: err}
	}
	backups := make([]retention.Backup, len(records))
	for i, r := range records {
		backups[i] = retention.Backup{Filename: r.Filename, Timestamp: r.Timestamp}
	}

	policy := resolveRetentionPolicy(cfg, target)
	prune := retention.Apply(policy, backups)

	if len(prune) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "nothing to prune for target %q\n", target)
		return nil
	}
	for _, name := range prune {
		fmt.Fprintln(cmd.OutOrStdout(), name)
	}

	if !apply {
		return nil
	}

	t, ok := cfg.Targets[target]
	storageName := t.Storage
	if storageName == "" {
		storageName = cfg.Defaults.Storage
	}
	if !ok || storageName == "" {
		return exitError{code: 2, err: fmt.Errorf("cli: target %q has no configured storage backend to delete from", target)}
	}
	spec, ok := cfg.Storage[storageName]
	if !ok {
		return exitError{code: 2, err: fmt.Errorf("cli: unknown storage backend %q", storageName)}
	}
	factory, ok := storage.Get(spec.Type)
	if !ok {
		return exitError{code: 2, err: fmt.Errorf("cli: unknown storage type %q", spec.Type)}
	}
	backend, err := factory(map[string]string{"path": spec.Path, "bucket": spec.Bucket, "region": spec.Region, "prefix": spec.Prefix})
	if err != nil {
		return exitError{code: 1, err: err}
	}
	for _, name := range prune {
		if err := backend.Delete(cmd.Context(), name); err != nil {
			return exitError{code: 1, err: err}
		}
		if err := db.DeleteBackup(cmd.Context(), name); err != nil {
			return exitError{code: 1, err: err}
		}
	}
	return nil
}

func newRetentionPreviewCommand() *cobra.Command {
	var target, stateDBPath string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Show what retention would delete without deleting it",
		RunE: func(cmd *cobra.Command, args []string) error {
			return retentionRun(cmd, target, stateDBPath, false)
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target name (required)")
	cmd.Flags().StringVar(&stateDBPath, "state-db", "", "Override the state DB path (default: config.DefaultStateDBPath)")
	mustMarkFlagRequired(cmd, "target")
	return cmd
}

func newRetentionApplyCommand() *cobra.Command {
	var target, stateDBPath string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Enforce retention policy now",
		RunE: func(cmd *cobra.Command, args []string) error {
			return retentionRun(cmd, target, stateDBPath, true)
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target name (required)")
	cmd.Flags().StringVar(&stateDBPath, "state-db", "", "Override the state DB path (default: config.DefaultStateDBPath)")
	mustMarkFlagRequired(cmd, "target")
	return cmd
}
