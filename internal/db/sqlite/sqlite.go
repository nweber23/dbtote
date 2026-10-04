package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/nweber23/dbtote/internal/driver"
)

func init() {
	driver.Register("sqlite", New)
}

// Engine's cfg.Database is the .db file path — SQLite has no host/port/
// user/password, so it reuses the field every other engine already uses
// for "what to back up" instead of introducing a second convention.
type Engine struct {
	cfg driver.ConnectionConfig
	db  *sql.DB
}

func New(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
	e := &Engine{cfg: cfg}
	return e, e, e
}

func (e *Engine) Connect(ctx context.Context, cfg driver.ConnectionConfig) error {
	e.cfg = cfg
	db, err := sql.Open("sqlite", cfg.Database)
	if err != nil {
		return fmt.Errorf("sqlite: open %q: %w", cfg.Database, err)
	}
	e.db = db
	return nil
}

func (e *Engine) Ping(ctx context.Context) error {
	if e.db == nil {
		return fmt.Errorf("sqlite: Ping called before Connect")
	}
	return e.db.PingContext(ctx)
}

func (e *Engine) Close() error {
	if e.db == nil {
		return nil
	}
	return e.db.Close()
}

// Backup uses SQLite's own "VACUUM INTO" (SQLite >= 3.27), which produces
// a transactionally consistent full copy safe under concurrent writers —
// the online-backup-API equivalent, without needing a CGo binding of
// sqlite3_backup_init.
func (e *Engine) Backup(ctx context.Context, opts driver.BackupOptions) (driver.BackupResult, error) {
	db, err := sql.Open("sqlite", opts.Database)
	if err != nil {
		return driver.BackupResult{}, fmt.Errorf("sqlite: open %q: %w", opts.Database, err)
	}
	defer db.Close()

	// Without a busy timeout, SQLite's default is to fail a locked access
	// immediately (SQLITE_BUSY) rather than wait — exactly the "concurrent
	// writers" case this backup needs to tolerate. This makes VACUUM INTO
	// retry for up to 5s instead of erroring the instant a writer holds
	// the lock.
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return driver.BackupResult{}, fmt.Errorf("sqlite: set busy_timeout: %w", err)
	}

	// VACUUM INTO refuses to write to a path that already exists, so the
	// destination can't be created up front the way os.CreateTemp normally
	// would be used — it has to not exist right before SQLite creates it.
	// A private 0700 directory closes that window: even though the file
	// itself briefly doesn't exist, nothing else on the machine can reach
	// this directory to race the creation or read the backup afterward.
	tmpDir, err := os.MkdirTemp("", "dbtote-sqlite-backup-*")
	if err != nil {
		return driver.BackupResult{}, fmt.Errorf("sqlite: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "backup.db")

	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", tmpPath); err != nil {
		return driver.BackupResult{}, fmt.Errorf("sqlite: VACUUM INTO: %w", err)
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return driver.BackupResult{}, fmt.Errorf("sqlite: open backup copy: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(opts.Output, f)
	if err != nil {
		return driver.BackupResult{}, fmt.Errorf("sqlite: stream backup copy: %w", err)
	}
	return driver.BackupResult{BytesWritten: n}, nil
}

func (e *Engine) SupportsIncremental() bool  { return false }
func (e *Engine) SupportsDifferential() bool { return false }
func (e *Engine) IncrementalBasis() driver.IncrementalBasisKind {
	return driver.BasisNone
}

// Restore is a plain file copy to opts.Database, written atomically via a
// temp file + rename so a reader never observes a half-written .db file.
func (e *Engine) Restore(ctx context.Context, opts driver.RestoreOptions) error {
	tmp := opts.Database + ".dbtote-restore-tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("sqlite: create temp restore file: %w", err)
	}
	if _, err := io.Copy(f, opts.Input); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("sqlite: write restore file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("sqlite: close restore file: %w", err)
	}
	if err := os.Rename(tmp, opts.Database); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("sqlite: rename into place: %w", err)
	}
	return nil
}

// SupportsSelective is false: a single-file database has no per-table
// restore via this interface — ATTACH + INSERT SELECT is a possible
// future feature, not built here.
func (e *Engine) SupportsSelective() bool { return false }
