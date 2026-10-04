package mongodb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/nweber23/dbtote/internal/driver"
)

// credentialPattern matches the userinfo portion of a MongoDB connection
// string (scheme://user:pass@) so it can be scrubbed from mongodump/
// mongorestore output before it's wrapped into an error — both tools echo
// the full URI, including the password, back into their own stderr on a
// connection failure.
var credentialPattern = regexp.MustCompile(`://[^@/\s]*@`)

func redactURI(s string) string {
	return credentialPattern.ReplaceAllString(s, "://***@")
}

// writeURIConfig writes the connection URI to a config file instead of
// passing --uri on the command line: argv is world-readable on Linux via
// /proc/<pid>/cmdline (and via `ps`), which would otherwise expose the
// embedded credentials to any other local user for as long as the process
// runs. The directory is created 0700 so nothing but this process can read
// the file in between.
func writeURIConfig(uri string) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "dbtote-mongo-*")
	if err != nil {
		return "", nil, fmt.Errorf("mongodb: create config dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(dir) }

	path = filepath.Join(dir, "config.yaml")
	contents := "uri: " + strconv.Quote(uri) + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("mongodb: write config file: %w", err)
	}
	return path, cleanup, nil
}

func init() {
	driver.Register("mongodb", New)
}

// Engine reads its connection string from cfg.Extra["uri"] — MongoDB
// targets configure a full URI via uri_env rather than separate
// host/port/user/password fields.
type Engine struct {
	cfg    driver.ConnectionConfig
	client *mongo.Client
}

func New(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
	e := &Engine{cfg: cfg}
	return e, e, e
}

func (e *Engine) Connect(ctx context.Context, cfg driver.ConnectionConfig) error {
	e.cfg = cfg
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.Extra["uri"]))
	if err != nil {
		return fmt.Errorf("mongodb: connect: %w", err)
	}
	e.client = client
	return nil
}

func (e *Engine) Ping(ctx context.Context) error {
	if e.client == nil {
		return fmt.Errorf("mongodb: Ping called before Connect")
	}
	return e.client.Ping(ctx, nil)
}

func (e *Engine) Close() error {
	if e.client == nil {
		return nil
	}
	return e.client.Disconnect(context.Background())
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

// Backup shells out to mongodump in archive mode, which produces a
// single streamable file (unlike mongodump's default directory-of-BSON
// output) — exactly what the dump->compress->encrypt->store pipeline needs.
func (e *Engine) Backup(ctx context.Context, opts driver.BackupOptions) (driver.BackupResult, error) {
	configPath, cleanup, err := writeURIConfig(e.cfg.Extra["uri"])
	if err != nil {
		return driver.BackupResult{}, err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, "mongodump",
		"--config="+configPath,
		"--db="+opts.Database,
		"--archive",
	)
	counting := &countingWriter{w: opts.Output}
	cmd.Stdout = counting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return driver.BackupResult{}, fmt.Errorf("mongodump: %w: %s", err, redactURI(stderr.String()))
	}
	return driver.BackupResult{BytesWritten: counting.n}, nil
}

func (e *Engine) SupportsIncremental() bool  { return true }
func (e *Engine) SupportsDifferential() bool { return true }
func (e *Engine) IncrementalBasis() driver.IncrementalBasisKind {
	return driver.BasisOplog
}

func (e *Engine) Restore(ctx context.Context, opts driver.RestoreOptions) error {
	configPath, cleanup, err := writeURIConfig(e.cfg.Extra["uri"])
	if err != nil {
		return err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, "mongorestore",
		"--config="+configPath,
		"--nsInclude="+opts.Database+".*",
		"--archive",
		"--drop",
	)
	cmd.Stdin = opts.Input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongorestore: %w: %s", err, redactURI(stderr.String()))
	}
	return nil
}

func (e *Engine) SupportsSelective() bool { return true }
