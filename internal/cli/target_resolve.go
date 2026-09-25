package cli

import (
	"github.com/spf13/cobra"

	"github.com/nweber23/dbtote/internal/config"
	"github.com/nweber23/dbtote/internal/driver"
)

// resolvedTarget is what backup.go and restore.go both need after
// reconciling CLI flags against an optional config target.
type resolvedTarget struct {
	Engine      string
	Connection  driver.ConnectionConfig
	PasswordEnv string
	StoragePath string
	StorageName string
	Recipients  []string
}

func resolveTarget(cmd *cobra.Command, target, host, user, database string, port int, passwordEnv, outputOrStoragePath, recipient string) resolvedTarget {
	rt := resolvedTarget{
		Engine:      "mysql",
		Connection:  driver.ConnectionConfig{Host: host, Port: port, User: user, Database: database},
		PasswordEnv: passwordEnv,
		StoragePath: outputOrStoragePath,
	}
	if recipient != "" {
		rt.Recipients = []string{recipient}
	}

	path, err := resolveConfigPath()
	if err != nil {
		return rt
	}
	cfg, err := config.Load(path)
	if err != nil {
		return rt
	}
	t, ok := cfg.Targets[target]
	if !ok {
		return rt
	}

	rt.Engine = t.Engine
	if !cmd.Flags().Changed("host") {
		rt.Connection.Host = t.Host
	}
	if !cmd.Flags().Changed("port") && t.Port != 0 {
		rt.Connection.Port = t.Port
	}
	if !cmd.Flags().Changed("user") {
		rt.Connection.User = t.User
	}
	if !cmd.Flags().Changed("database") {
		rt.Connection.Database = t.Database
	}
	if !cmd.Flags().Changed("password-env") {
		rt.PasswordEnv = t.PasswordEnv
	}
	if !cmd.Flags().Changed("output") && t.Storage != "" {
		if spec, ok := cfg.Storage[t.Storage]; ok {
			rt.StoragePath = spec.Path
			rt.StorageName = t.Storage
		}
	}
	if !cmd.Flags().Changed("recipient") {
		rt.Recipients = cfg.Encryption.Recipients
	}
	return rt
}
