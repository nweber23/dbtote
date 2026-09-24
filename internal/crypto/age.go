package crypto

import (
	"fmt"
	"io"

	"filippo.io/age"
)

func NewEncryptWriter(w io.Writer, recipients []string) (io.WriteCloser, error) {
	parsed := make([]age.Recipient, 0, len(recipients))
	for _, r := range recipients {
		id, err := age.ParseX25519Recipient(r)
		if err != nil {
			return nil, fmt.Errorf("crypto: parse recipient %q: %w", r, err)
		}
		parsed = append(parsed, id)
	}
	wc, err := age.Encrypt(w, parsed...)
	if err != nil {
		return nil, fmt.Errorf("crypto: , age.Encrypt: %w", err)
	}
	return wc, nil
}

func NewDecryptReader(r io.Reader, identity string) (io.Reader, error) {
	id, err := age.ParseX25519Identity(identity)
	if err != nil {
		return nil, fmt.Errorf("crypto: parse identity: %w", err)
	}
	plaintext, err := age.Decrypt(r, id)
	if err != nil {
		return nil, fmt.Errorf("crypto: age.Decrypt: %w", err)
	}
	return plaintext, nil
}
