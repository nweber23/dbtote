package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// BackupMeta describes one stored backup, reconstructed identically
type BackupMeta struct {
	Name      string
	Target    string
	Type      string
	Size      int64
	Timestamp time.Time
}

type Backend interface {
	Store(ctx context.Context, name string, r io.Reader) error
	Retrieve(ctx context.Context, name string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]BackupMeta, error)
	Delete(ctx context.Context, name string) error
}

type Factory func(cfg map[string]string) (Backend, error)

var registry = map[string]Factory{}

func Register(kind string, f Factory) {
	if _, exists := registry[kind]; exists {
		panic(fmt.Sprintf("storage: backend %q already registered", kind))
	}
	registry[kind] = f
}

func Get(kind string) (Factory, bool) {
	f, ok := registry[kind]
	return f, ok
}
