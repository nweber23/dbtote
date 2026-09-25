package backupjob

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/nweber23/dbtote/internal/compress"
	"github.com/nweber23/dbtote/internal/crypto"
	"github.com/nweber23/dbtote/internal/driver"
	"github.com/nweber23/dbtote/internal/retention"
	"github.com/nweber23/dbtote/internal/storage"
)

type Error struct {
	Stage string
	Err   error
}

func (e *Error) Error() string { return fmt.Sprintf("backupjob: %s: %v", e.Stage, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

type Notifier interface {
	Notify(ctx context.Context, message string) error
}

type Options struct {
	TargetName      string
	Engine          string
	Connection      driver.ConnectionConfig
	Type            string // "full" (default) | "incremental" | "differential" — only "full" is implemented before Phase 7
	DumpFileExt     string
	Compress        string
	Encrypt         bool
	Recipients      []string
	Storage         storage.Backend
	StorageName     string // the configured storage backend's name, recorded in BackupRecord
	State           *StateDB
	RetentionPolicy *retention.Policy
	DryRun          bool
	Notifier        Notifier
	NotifyOn        []string // subset of {"success", "failure"}
	Now             func() time.Time
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

	backupType := opts.Type
	if backupType == "" {
		backupType = "full"
	}

	logAttrs := []any{"target", opts.TargetName, "type", backupType}

	if err := conn.Connect(ctx, opts.Connection); err != nil {
		logger.Error("backup failed", append(logAttrs, "stage", "connect", "error", err.Error())...)
		notifyFailure(ctx, opts, logger, logAttrs, "connect", err)
		return Result{}, &Error{Stage: "connect", Err: err}
	}
	defer conn.Close()

	if err := conn.Ping(ctx); err != nil {
		logger.Error("backup failed", append(logAttrs, "stage", "test-connection", "error", err.Error())...)
		notifyFailure(ctx, opts, logger, logAttrs, "test-connection", err)
		return Result{}, &Error{Stage: "test-connection", Err: err}
	}

	extChain := "." + opts.DumpFileExt + compress.Ext(opts.Compress)
	if opts.Encrypt {
		extChain += ".age"
	}
	filename := BuildFilename(opts.TargetName, backupType, now(), extChain)

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

	counted := &countingReader{r: pr}
	if err := opts.Storage.Store(ctx, filename, counted); err != nil {
		<-dumpErrCh
		logger.Error("backup failed", append(logAttrs, "stage", "store", "error", err.Error())...)
		notifyFailure(ctx, opts, logger, logAttrs, "store", err)
		return Result{}, &Error{Stage: "store", Err: err}
	}
	if err := <-dumpErrCh; err != nil {
		logger.Error("backup failed", append(logAttrs, "stage", "dump", "error", err.Error())...)
		notifyFailure(ctx, opts, logger, logAttrs, "dump", err)
		return Result{}, &Error{Stage: "dump", Err: err}
	}

	duration := time.Since(start)
	logger.Info("backup completed", append(logAttrs, "duration_ms", duration.Milliseconds(), "status", "success")...)

	if opts.State != nil {
		if err := opts.State.RecordBackup(ctx, BackupRecord{
			Filename:  filename,
			Target:    opts.TargetName,
			Type:      backupType,
			Storage:   opts.StorageName,
			Size:      counted.n,
			Timestamp: now(),
		}); err != nil {
			logger.Warn("failed to record backup metadata", append(logAttrs, "error", err.Error())...)
		}

		if opts.RetentionPolicy != nil && !opts.DryRun {
			applyRetention(ctx, opts, logger, logAttrs)
		}
	}

	if opts.Notifier != nil && slices.Contains(opts.NotifyOn, "success") {
		if err := opts.Notifier.Notify(ctx, fmt.Sprintf("dbtote: backup succeeded for %s (%s)", opts.TargetName, filename)); err != nil {
			logger.Warn("notification failed", append(logAttrs, "error", err.Error())...)
		}
	}

	return Result{Filename: filename, BytesWritten: counted.n, Duration: duration}, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func notifyFailure(ctx context.Context, opts Options, logger *slog.Logger, logAttrs []any, stage string, cause error) {
	if opts.Notifier == nil || !slices.Contains(opts.NotifyOn, "failure") {
		return
	}
	msg := fmt.Sprintf("dbtote: backup failed for %s at stage %q: %v", opts.TargetName, stage, cause)
	if err := opts.Notifier.Notify(ctx, msg); err != nil {
		logger.Warn("failure notification failed", append(logAttrs, "error", err.Error())...)
	}
}

// applyRetention prunes backups beyond opts.RetentionPolicy for this
// target from both the state index and the storage backend. A retention
// failure is logged but never fails the backup job itself — the backup
// already succeeded by the time retention runs.
func applyRetention(ctx context.Context, opts Options, logger *slog.Logger, logAttrs []any) {
	records, err := opts.State.ListBackups(ctx, ListFilter{Target: opts.TargetName})
	if err != nil {
		logger.Warn("retention: failed to list backups", append(logAttrs, "error", err.Error())...)
		return
	}
	backups := make([]retention.Backup, len(records))
	for i, r := range records {
		backups[i] = retention.Backup{Filename: r.Filename, Timestamp: r.Timestamp}
	}
	prune := retention.Apply(*opts.RetentionPolicy, backups)
	for _, name := range prune {
		if err := opts.Storage.Delete(ctx, name); err != nil {
			logger.Warn("retention: failed to delete from storage", append(logAttrs, "filename", name, "error", err.Error())...)
			continue
		}
		if err := opts.State.DeleteBackup(ctx, name); err != nil {
			logger.Warn("retention: failed to delete state record", append(logAttrs, "filename", name, "error", err.Error())...)
		}
	}
}
