package secrets

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	values map[string]string
}

func (f *fakeStore) Get(ctx context.Context, key string) (string, error) {
	v, ok := f.values[key]
	if !ok {
		return "", errors.New("fakeStore: not found")
	}
	return v, nil
}
func (f *fakeStore) Set(ctx context.Context, key, value string) error {
	f.values[key] = value
	return nil
}
func (f *fakeStore) Delete(ctx context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func TestResolvePassword_PrefersExplicitFlag(t *testing.T) {
	store := &fakeStore{values: map[string]string{"t": "from-keyring"}}
	got, err := ResolvePassword(context.Background(), store, ResolveOptions{
		ExplicitPassword: "from-flag",
		KeyringKey:       "t",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-flag" {
		t.Errorf("got %q, want %q", got, "from-flag")
	}
}

func TestResolvePassword_FallsBackToKeyring(t *testing.T) {
	store := &fakeStore{values: map[string]string{"t": "from-keyring"}}
	got, err := ResolvePassword(context.Background(), store, ResolveOptions{
		KeyringKey: "t",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-keyring" {
		t.Errorf("got %q, want %q", got, "from-keyring")
	}
}

func TestResolvePassword_FallsBackToPrompt(t *testing.T) {
	store := &fakeStore{values: map[string]string{}}
	got, err := ResolvePassword(context.Background(), store, ResolveOptions{
		KeyringKey: "t",
		Prompt:     func() (string, error) { return "from-prompt", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-prompt" {
		t.Errorf("got %q, want %q", got, "from-prompt")
	}
}

func TestResolvePassword_HardErrorWhenNothingResolves(t *testing.T) {
	store := &fakeStore{values: map[string]string{}}
	_, err := ResolvePassword(context.Background(), store, ResolveOptions{KeyringKey: "t"})
	if err == nil {
		t.Error("expected an error when no source resolves and no prompt is available")
	}
}
