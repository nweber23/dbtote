package crypto

import (
	"bytes"
	"io"
	"testing"

	"filippo.io/age"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}

	var buf bytes.Buffer
	w, err := NewEncryptWriter(&buf, []string{identity.Recipient().String()})
	if err != nil {
		t.Fatalf("NewEncryptWriter: %v", err)
	}
	if _, err := w.Write([]byte("secret dump bytes")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := NewDecryptReader(&buf, identity.String())
	if err != nil {
		t.Fatalf("NewDecryptReader: %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "secret dump bytes" {
		t.Errorf("got %q, want %q", got, "secret dump bytes")
	}
}

func TestNewEncryptWriter_InvalidRecipient(t *testing.T) {
	if _, err := NewEncryptWriter(&bytes.Buffer{}, []string{"not-a-real-key"}); err == nil {
		t.Error("expected an error for an invalid recipient")
	}
}

func TestNewDecryptReader_InvalidIdentity(t *testing.T) {
	if _, err := NewDecryptReader(&bytes.Buffer{}, "not-a-real-identity"); err == nil {
		t.Error("expected an error for an invalid identity")
	}
}
