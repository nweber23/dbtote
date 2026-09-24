package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

const validYAML = `
version: 1
defaults:
  compress: gzip
  encrypt: false
  storage: local-main
storage:
  local-main:
    type: local
    path: /var/backups/dbtote
targets:
  prod-mysql:
    engine: mysql
    host: db1.internal
    port: 3306
    user: backup_svc
    database: app_production
    storage: local-main
schedules:
  - id: nightly
    target: prod-mysql
    cron: "0 2 * * *"
    type: full
`

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoad_ParsesValidConfig(t *testing.T) {
	path := writeTemp(t, validYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.Storage != "local-main" {
		t.Errorf("Defaults.Storage = %q, want local-main", cfg.Defaults.Storage)
	}
	target, ok := cfg.Targets["prod-mysql"]
	if !ok {
		t.Fatal("expected targets.prod-mysql")
	}
	if target.Engine != "mysql" || target.Host != "db1.internal" {
		t.Errorf("unexpected target: %+v", target)
	}
	if len(cfg.Schedules) != 1 || cfg.Schedules[0].Cron != "0 2 * * *" {
		t.Errorf("unexpected schedules: %+v", cfg.Schedules)
	}
}

func TestValidate_CatchesUnknownStorageReference(t *testing.T) {
	path := writeTemp(t, `
version: 1
defaults:
  storage: does-not-exist
storage:
  local-main:
    type: local
    path: /tmp
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected Validate to fail for an unknown storage reference")
	}
}

func TestValidate_CatchesInvalidAgeRecipient(t *testing.T) {
	path := writeTemp(t, `
version: 1
encryption:
  recipients:
    - not-a-real-age-key
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected Validate to fail for an invalid age recipient")
	}
}

func TestValidate_CatchesInvalidCron(t *testing.T) {
	path := writeTemp(t, `
version: 1
schedules:
  - id: bad
    target: x
    cron: "not a cron expression"
    type: full
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected Validate to fail for an invalid cron expression")
	}
}

func TestValidate_AcceptsWellFormedConfig(t *testing.T) {
	identity, _ := age.GenerateX25519Identity()
	path := writeTemp(t, `
version: 1
encryption:
  recipients:
    - `+identity.Recipient().String()+`
storage:
  local-main:
    type: local
    path: /tmp
schedules:
  - id: nightly
    target: t
    cron: "0 2 * * *"
    type: full
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected Validate to pass, got: %v", err)
	}
}

type fakeStore struct{ values map[string]string }

func (f *fakeStore) Get(ctx context.Context, key string) (string, error) {
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}
func (f *fakeStore) Set(ctx context.Context, key, value string) error {
	f.values[key] = value
	return nil
}
func (f *fakeStore) Delete(ctx context.Context, key string) error { delete(f.values, key); return nil }

func TestValidateCredentials_FlagsMissingCredential(t *testing.T) {
	cfg := &Config{Targets: map[string]Target{"prod-mysql": {Engine: "mysql"}}}
	store := &fakeStore{values: map[string]string{}}
	if err := cfg.ValidateCredentials(context.Background(), store); err == nil {
		t.Error("expected ValidateCredentials to fail when no credential resolves")
	}
}

func TestValidateCredentials_PassesWithKeyringEntry(t *testing.T) {
	cfg := &Config{Targets: map[string]Target{"prod-mysql": {Engine: "mysql"}}}
	store := &fakeStore{values: map[string]string{"prod-mysql": "s3cr3t"}}
	if err := cfg.ValidateCredentials(context.Background(), store); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateCredentials_SkipsSQLiteAndURIEnvTargets(t *testing.T) {
	cfg := &Config{Targets: map[string]Target{
		"local-sqlite": {Engine: "sqlite"},
		"prod-mongo":   {Engine: "mongodb", URIEnv: "MONGO_URI"},
	}}
	store := &fakeStore{values: map[string]string{}}
	if err := cfg.ValidateCredentials(context.Background(), store); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDefaultPath_ReturnsAnAbsolutePath(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected an absolute path, got %q", p)
	}
}
