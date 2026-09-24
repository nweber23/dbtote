package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os/exec"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/nweber23/dbtote/internal/driver"
)

func init() {
	driver.Register("postgres", New)
}

type Engine struct {
	cfg driver.ConnectionConfig
	db  *sql.DB
}

func New(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
	e := &Engine{cfg: cfg}
	return e, e, e
}

func (e *Engine) dsn(cfg driver.ConnectionConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/" + cfg.Database,
	}
	sslmode := cfg.Extra["sslmode"]
	if sslmode == "" {
		sslmode = "disable"
	}
	q := url.Values{"sslmode": {sslmode}}
	u.RawQuery = q.Encode()
	return u.String()
}

func (e *Engine) Connect(ctx context.Context, cfg driver.ConnectionConfig) error {
	e.cfg = cfg
	db, err := sql.Open("pgx", e.dsn(cfg))
	if err != nil {
		return fmt.Errorf("postgres: open: %w", err)
	}
	e.db = db
	return nil
}

func (e *Engine) Ping(ctx context.Context) error {
	if e.db == nil {
		return fmt.Errorf("postgres: Ping called before Connect")
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
	cmd := exec.CommandContext(ctx, "pg_dump",
		"--host="+e.cfg.Host,
		fmt.Sprintf("--port=%d", e.cfg.Port),
		"--username="+e.cfg.User,
		"--format=custom",
		opts.Database,
	)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+e.cfg.Password)

	counting := &countingWriter{w: opts.Output}
	cmd.Stdout = counting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return driver.BackupResult{}, fmt.Errorf("pg_dump: %w: %s", err, stderr.String())
	}
	return driver.BackupResult{BytesWritten: counting.n}, nil
}

func (e *Engine) SupportsIncremental() bool  { return true }
func (e *Engine) SupportsDifferential() bool { return true }
func (e *Engine) IncrementalBasis() driver.IncrementalBasisKind {
	return driver.BasisWAL
}

func (e *Engine) Restore(ctx context.Context, opts driver.RestoreOptions) error {
	cmd := exec.CommandContext(ctx, "pg_restore",
		"--host="+e.cfg.Host,
		fmt.Sprintf("--port=%d", e.cfg.Port),
		"--username="+e.cfg.User,
		"--dbname="+opts.Database,
		"--clean",
		"--if-exists",
	)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+e.cfg.Password)
	cmd.Stdin = opts.Input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore: %w: %s", err, stderr.String())
	}
	return nil
}

func (e *Engine) SupportsSelective() bool { return true }
