package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nweber23/dbtote/internal/storage"
)

func init() {
	storage.Register("local", func(cfg map[string]string) (storage.Backend, error) {
		path := cfg["path"]
		if path == "" {
			return nil, fmt.Errorf("local: config missing required %q", "path")
		}
		return New(path), nil
	})
}

type Backend struct {
	root string
}

func New(root string) *Backend {
	return &Backend{root: root}
}

// Store writes r to / atomically: it writes to a same-directory
// temp file first, then os.Rename()s into place, so nothing ever observes
func (b *Backend) Store(ctx context.Context, name string, r io.Reader) error {
	if err := os.MkdirAll(b.root, 0o755); err != nil {
		return fmt.Errorf("local: mkdir %q: %w", b.root, err)
	}
	final := filepath.Join(b.root, name)
	tmp := final + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("local: create temp file: %w", err)
	}
	if _, copyErr := io.Copy(f, r); copyErr != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("local: write: %w", copyErr)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("local: close temp file: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("local: rename into place: %w", err)
	}
	return nil
}

func (b *Backend) Retrieve(ctx context.Context, name string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(b.root, name))
	if err != nil {
		return nil, fmt.Errorf("local: open %q: %w", name, err)
	}
	return f, nil
}

func (b *Backend) List(ctx context.Context, prefix string) ([]storage.BackupMeta, error) {
	entries, err := os.ReadDir(b.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("local: read dir %q: %w", b.root, err)
	}

	var metas []storage.BackupMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		metas = append(metas, storage.BackupMeta{
			Name:      e.Name(),
			Size:      info.Size(),
			Timestamp: info.ModTime(),
		})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Timestamp.Before(metas[j].Timestamp) })
	return metas, nil
}

func (b *Backend) Delete(ctx context.Context, name string) error {
	if err := os.Remove(filepath.Join(b.root, name)); err != nil {
		return fmt.Errorf("local: delete %q: %w", name, err)
	}
	return nil
}
