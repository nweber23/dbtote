package secrets

import (
	"context"
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "dbtote"

type KeyringStore struct{}

func NewKeyringStore() *KeyringStore {
	return &KeyringStore{}
}

func (k *KeyringStore) Get(ctx context.Context, key string) (string, error) {
	v, err := keyring.Get(serviceName, key)
	if err != nil {
		return "", fmt.Errorf("secrets: keyring get %q: %w", key, err)
	}
	return v, nil
}

func (k *KeyringStore) Set(ctx context.Context, key, value string) error {
	err := keyring.Set(serviceName, key, value)
	if err != nil {
		return fmt.Errorf("secrets: keyring set %q: %w", key, err)
	}
	return nil
}

func (k *KeyringStore) Delete(ctx context.Context, key string) error {
	err := keyring.Delete(serviceName, key)
	if err != nil {
		return fmt.Errorf("secrets: keyring delete %q: %w", key, err)
	}
	return nil
}
