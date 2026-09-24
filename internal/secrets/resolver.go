package secrets

import (
	"context"
	"fmt"
	"os"
)

type ResolveOptions struct {
	ExplicitPassword string
	EnvVarName       string
	KeyringKey       string
	Prompt           func() (string, error)
}

func ResolvePassword(ctx context.Context, store Store, opts ResolveOptions) (string, error) {
	if opts.ExplicitPassword != "" {
		return opts.ExplicitPassword, nil
	}
	if opts.EnvVarName != "" {
		if v := os.Getenv(opts.EnvVarName); v != "" {
			return v, nil
		}
	}
	if v, err := store.Get(ctx, opts.KeyringKey); err == nil && v != "" {
		return v, nil
	}
	if opts.Prompt != nil {
		return opts.Prompt()
	}
	return "", fmt.Errorf("secrets: no password source resolved for %q (checked flag, env, keyring; no TTY prompt available)", opts.KeyringKey)
}
