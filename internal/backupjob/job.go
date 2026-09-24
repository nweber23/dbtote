package backupjob

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/nweber23/dbtote/internal/compress"
	"github.com/nweber23/dbtote/internal/crypto"
	"github.com/nweber23/dbtote/internal/driver"
	"github.com/nweber23/dbtote/internal/storage"
)

type Error struct {
	Stage string
	Err   error
}

func (e *Error) Error() string { return fmt.Sprintf("backupjob: %s: %v", e.Stage, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

type Options struct {
	TargetName  string
	Engine      string
	Connection  driver.ConnectionConfig
	DumpFileExt string // base extension before compression/encryption, e.g. "sql"
	Compress    string // "none" | "gzip"
	Encrypt     bool
	Recipients  []string
	Storage     storage.Backend
	Now         func() time.Time // injectable clock; nil defaults to time.Now
}

type Result struct {
	Filename     string
	BytesWritten int64
	Duration     time.Duration
}

// Run executes one full backup: resolve engine -> test connection -> dump
// -> compress -> encrypt -> store -> log, streamed end-to-end through
// io.Pipe so the whole backup is never buffered in memory at once
func Run(ctx context.Context, opts Options, logger *slog.Logger) (Result, error) {
	start := time.Now()
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	factory, ok := driver.Get(opts.Engine)
	if !ok {
		return Result{}, fmt.Errorf("backupjob: unknown engine %q", opts.Engine)
	}
	conn, backuper, _ := factory(opts.Connection)

	logAttrs := []any{"target", opts.TargetName, "type", "full"}

	if err := conn.Connect(ctx, opts.Connection); err != nil {
		logger.Error("backup failed", append(logAttrs, "stage", "connect", "error", err.Error())...)
		return Result{}, &Error{Stage: "connect", Err: err}
	}
	defer conn.Close()

	if err := conn.Ping(ctx); err != nil {
		logger.Error("backup failed", append(logAttrs, "stage", "test-connection", "error", err.Error())...)
		return Result{}, &Error{Stage: "test-connection", Err: err}
	}

	extChain := "." + opts.DumpFileExt + compress.Ext(opts.Compress)
	if opts.Encrypt {
		extChain += ".age"
	}
	filename := BuildFilename(opts.TargetName, "full", now(), extChain)

	pr, pw := io.Pipe()

	dumpErrCh := make(chan error, 1)
	go func() {
		var closers []io.Closer
		var target io.Writer = pw
		if opts.Encrypt {
			encWriter, err := crypto.NewEncryptWriter(pw, opts.Recipients)
			if err != nil {
				pw.CloseWithError(err)
				dumpErrCh <- fmt.Errorf("backupjob: encrypt writer: %w", err)
				return
			}
			target = encWriter
			closers = append(closers, encWriter)
		}
		compWriter, err := compress.NewWriter(opts.Compress, target)
		if err != nil {
			pw.CloseWithError(err)
			dumpErrCh <- fmt.Errorf("backupjob: compress writer: %w", err)
			return
		}
		closers = append([]io.Closer{compWriter}, closers...) // compress closes (flushes) first, then encrypt

		_, dumpErr := backuper.Backup(ctx, driver.BackupOptions{Database: opts.Connection.Database, Output: compWriter})
		var closeErr error
		for _, c := range closers {
			if err := c.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		err = dumpErr
		if err == nil {
			err = closeErr
		}
		pw.CloseWithError(err)
		dumpErrCh <- err
	}()

	if err := opts.Storage.Store(ctx, filename, pr); err != nil {
		<-dumpErrCh
		logger.Error("backup failed", append(logAttrs, "stage", "store", "error", err.Error())...)
		return Result{}, &Error{Stage: "store", Err: err}
	}
	if err := <-dumpErrCh; err != nil {
		logger.Error("backup failed", append(logAttrs, "stage", "dump", "error", err.Error())...)
		return Result{}, &Error{Stage: "dump", Err: err}
	}

	duration := time.Since(start)
	logger.Info("backup completed", append(logAttrs, "duration_ms", duration.Milliseconds(), "status", "success")...)

	return Result{Filename: filename, Duration: duration}, nil
}
