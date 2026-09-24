package driver

import (
	"context"
	"io"
)

// IncrementalBasisKind names what an incremental backup for a given engine is anchored to.
type IncrementalBasisKind int

const (
	BasisNone IncrementalBasisKind = iota
	BasisBinlog
	BasisWAL
	BasisOplog
)

func (k IncrementalBasisKind) String() string {
	switch k {
	case BasisBinlog:
		return "binlog"
	case BasisWAL:
		return "wal"
	case BasisOplog:
		return "oplog"
	default:
		return "none"
	}
}

type ConnectionConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Extra    map[string]string
}

type Connector interface {
	Connect(ctx context.Context, cfg ConnectionConfig) error
	Ping(ctx context.Context) error
	Close() error
}

type BackupOptions struct {
	Database string
	Output   io.Writer
}

type BackupResult struct {
	BytesWritten int64
}

type Backuper interface {
	Backup(ctx context.Context, opts BackupOptions) (BackupResult, error)
	SupportsIncremental() bool
	SupportsDifferential() bool
	IncrementalBasis() IncrementalBasisKind
}

type RestoreOptions struct {
	Database string
	Input    io.Reader
}

type Restorer interface {
	Restore(ctx context.Context, opts RestoreOptions) error
	SupportsSelective() bool
}
