package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRetrieveDelete(t *testing.T) {
	dir := t.TempDir()
	b := New(dir)
	ctx := context.Background()

	if err := b.Store(ctx, "backup1.sql", bytes.NewBufferString("dump contents")); err != nil {
		t.Fatalf("Store: %v", err)
	}

	r, err := b.Retrieve(ctx, "backup1.sql")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "dump contents" {
		t.Errorf("got %q, want %q", got, "dump contents")
	}

	metas, err := b.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 || metas[0].Name != "backup1.sql" {
		t.Errorf("unexpected List result: %+v", metas)
	}

	if err := b.Delete(ctx, "backup1.sql"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := b.Retrieve(ctx, "backup1.sql"); err == nil {
		t.Error("expected Retrieve to fail after Delete")
	}
}

func TestStore_NoPartialFileOnWriteError(t *testing.T) {
	dir := t.TempDir()
	b := New(dir)
	ctx := context.Background()

	errReader := &erroringReader{failAfter: 5}
	err := b.Store(ctx, "partial.sql", errReader)
	if err == nil {
		t.Fatal("expected Store to fail")
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		t.Errorf("expected no leftover file in %s, found %s", dir, e.Name())
	}
	if _, statErr := os.Stat(filepath.Join(dir, "partial.sql")); statErr == nil {
		t.Error("expected partial.sql to not exist")
	}
}

type erroringReader struct {
	failAfter int
	read      int
}

func (r *erroringReader) Read(p []byte) (int, error) {
	if r.read >= r.failAfter {
		return 0, errors.New("simulated write source failure")
	}
	n := copy(p, bytes.Repeat([]byte("x"), 1))
	r.read += n
	return n, nil
}