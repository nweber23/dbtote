package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"

	"github.com/nweber23/dbtote/internal/secrets"
)

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("DBTOTE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	return &cfg, nil
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "dbtote", "config.yaml"), nil
}

// DefaultStateDBPath returns the default location of dbtote's local
// backup-metadata SQLite index, alongside the default config file.
func DefaultStateDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "dbtote", "state.db"), nil
}

func (c *Config) Validate() error {
	var errs []string

	for name, t := range c.Targets {
		if t.Storage != "" {
			if _, ok := c.Storage[t.Storage]; !ok {
				errs = append(errs, fmt.Sprintf("targets.%s.storage %q is not defined under storage:", name, t.Storage))
			}
		}
	}
	if c.Defaults.Storage != "" {
		if _, ok := c.Storage[c.Defaults.Storage]; !ok {
			errs = append(errs, fmt.Sprintf("defaults.storage %q is not defined under storage:", c.Defaults.Storage))
		}
	}

	for _, r := range c.Encryption.Recipients {
		if _, err := age.ParseX25519Recipient(r); err != nil {
			errs = append(errs, fmt.Sprintf("encryption.recipients: invalid age key %q: %v", r, err))
		}
	}

	for _, s := range c.Schedules {
		if _, err := cron.ParseStandard(s.Cron); err != nil {
			errs = append(errs, fmt.Sprintf("schedules[%s].cron %q: %v", s.ID, s.Cron, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config: invalid:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func (c *Config) ValidateCredentials(ctx context.Context, store secrets.Store) error {
	var errs []string
	for name, t := range c.Targets {
		if t.Engine == "sqlite" || t.URIEnv != "" {
			continue
		}
		if _, err := secrets.ResolvePassword(ctx, store, secrets.ResolveOptions{
			EnvVarName: t.PasswordEnv,
			KeyringKey: name,
		}); err != nil {
			errs = append(errs, fmt.Sprintf("targets.%s: no resolvable credential (checked password_env and keyring): %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("config: invalid:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
