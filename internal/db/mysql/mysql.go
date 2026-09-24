package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os/exec"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nweber23/dbtote/internal/driver"
)

func init() {
	driver.Register("mysql", New)
}

type Engine struct {
	cfg driver.ConnectionConfig
	db  *sql.DB
}

// New returns the same *Engine as all three driver interfaces, following
// the pattern every other engine package in this codebase repeats.
func New(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
	e := &Engine{cfg: cfg}
	return e, e, e
}

func (e *Engine) Connect(ctx context.Context, cfg driver.ConnectionConfig) error {
	e.cfg = cfg
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("mysql: open: %w", err)
	}
	e.db = db
	return nil
}

func (e *Engine) Ping(ctx context.Context) error {
	if e.db == nil {
		return fmt.Errorf("mysql: Ping called before Connect")
	}
	return e.db.PingContext(ctx)
}

func (e *Engine) Close() error {
	if e.db == nil {
		return nil
	}
	return e.db.Close()
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func (e *Engine) Backup(ctx context.Context, opts driver.BackupOptions) (driver.BackupResult, error) {
	cmd := exec.CommandContext(ctx, "mysqldump",
		"--host="+e.cfg.Host,
		fmt.Sprintf("--port=%d", e.cfg.Port),
		"--user="+e.cfg.User,
		opts.Database,
	)
	cmd.Env = append(cmd.Env, "MYSQL_PWD="+e.cfg.Password)

	counting := &countingWriter{w: opts.Output}
	cmd.Stdout = counting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return driver.BackupResult{}, fmt.Errorf("mysqldump: %w: %s", err, stderr.String())
	}
	return driver.BackupResult{BytesWritten: counting.n}, nil
}

func (e *Engine) SupportsIncremental() bool  { return true }
func (e *Engine) SupportsDifferential() bool { return true }
func (e *Engine) IncrementalBasis() driver.IncrementalBasisKind {
	return driver.BasisBinlog
}

func (e *Engine) Restore(ctx context.Context, opts driver.RestoreOptions) error {
	cmd := exec.CommandContext(ctx, "mysql",
		"--host="+e.cfg.Host,
		fmt.Sprintf("--port=%d", e.cfg.Port),
		"--user="+e.cfg.User,
		opts.Database,
	)
	cmd.Env = append(cmd.Env, "MYSQL_PWD="+e.cfg.Password)
	cmd.Stdin = opts.Input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql restore: %w: %s", err, stderr.String())
	}
	return nil
}

func (e *Engine) SupportsSelective() bool { return true }
