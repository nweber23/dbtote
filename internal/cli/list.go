package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/backupjob"
	"github.com/nweber23/dbtote/internal/config"
	"github.com/nweber23/dbtote/internal/storage"
)

type listEntry struct {
	Name      string    `json:"name"`
	Target    string    `json:"target"`
	Type      string    `json:"type"`
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
}

func newListCommand() *cobra.Command {
	var (
		targetFilter  string
		storageFilter string
		since         time.Duration
		asJSON        bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List known backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return exitError{code: 2, err: err}
			}
			cfg, err := config.Load(path)
			if err != nil {
				return exitError{code: 2, err: err}
			}

			storageName := storageFilter
			if storageName == "" {
				storageName = cfg.Defaults.Storage
			}
			spec, ok := cfg.Storage[storageName]
			if !ok {
				return exitError{code: 2, err: fmt.Errorf("cli: unknown storage backend %q", storageName)}
			}
			factory, ok := storage.Get(spec.Type)
			if !ok {
				return exitError{code: 2, err: fmt.Errorf("cli: unknown storage type %q", spec.Type)}
			}
			backend, err := factory(map[string]string{"path": spec.Path})
			if err != nil {
				return exitError{code: 2, err: err}
			}

			prefix := ""
			if targetFilter != "" {
				prefix = targetFilter + "_"
			}
			metas, err := backend.List(cmd.Context(), prefix)
			if err != nil {
				return exitError{code: 1, err: err}
			}

			cutoff := time.Time{}
			if since > 0 {
				cutoff = time.Now().Add(-since)
			}

			var entries []listEntry
			for _, m := range metas {
				target, backupType, ts, ok := backupjob.ParseFilename(m.Name)
				if !ok {
					continue
				}
				if !cutoff.IsZero() && ts.Before(cutoff) {
					continue
				}
				entries = append(entries, listEntry{Name: m.Name, Target: target, Type: backupType, Size: m.Size, Timestamp: ts})
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-12s %-6s %10d bytes  %s\n", e.Name, e.Target, e.Type, e.Size, e.Timestamp.Format(time.RFC3339))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&targetFilter, "target", "", "Filter by target name")
	cmd.Flags().StringVar(&storageFilter, "storage", "", "Storage backend name (default: config default)")
	cmd.Flags().DurationVar(&since, "since", 0, "Only show backups newer than this duration ago")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}
