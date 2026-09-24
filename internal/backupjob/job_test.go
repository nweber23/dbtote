package backupjob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/nweber23/dbtote/internal/driver"
	"github.com/nweber23/dbtote/internal/storage"
)

type stubConnector struct{ pingErr error }

func (s *stubConnector) Connect(ctx context.Context, cfg driver.ConnectionConfig) error { return nil }
func (s *stubConnector) Ping(ctx context.Context) error                                 { return s.pingErr }
func (s *stubConnector) Close() error                                                   { return nil }

type stubBackuper struct {
	payload []byte
	failErr error
}

func (s *stubBackuper) Backup(ctx context.Context, opts driver.BackupOptions) (driver.BackupResult, error) {
	if s.failErr != nil {
		return driver.BackupResult{}, s.failErr
	}
	n, err := opts.Output.Write(s.payload)
	return driver.BackupResult{BytesWritten: int64(n)}, err
}
func (s *stubBackuper) SupportsIncremental() bool                     { return false }
func (s *stubBackuper) SupportsDifferential() bool                    { return false }
func (s *stubBackuper) IncrementalBasis() driver.IncrementalBasisKind { return driver.BasisNone }

type stubStorage struct {
	stored map[string][]byte
}

func (s *stubStorage) Store(ctx context.Context, name string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if s.stored == nil {
		s.stored = map[string][]byte{}
	}
	s.stored[name] = b
	return nil
}
func (s *stubStorage) Retrieve(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (s *stubStorage) List(ctx context.Context, prefix string) ([]storage.BackupMeta, error) {
	return nil, nil
}
func (s *stubStorage) Delete(ctx context.Context, name string) error { return nil }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRun_StreamsThroughCompressAndStore(t *testing.T) {
	driver.Register("stub-run-1", func(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
		return &stubConnector{}, &stubBackuper{payload: []byte("dump bytes")}, nil
	})

	st := &stubStorage{}
	fixedTime := time.Date(2026, 9, 22, 2, 0, 0, 0, time.UTC)

	result, err := Run(context.Background(), Options{
		TargetName:  "unit-target",
		Engine:      "stub-run-1",
		DumpFileExt: "sql",
		Compress:    "gzip",
		Storage:     st,
		Now:         func() time.Time { return fixedTime },
	}, discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantFilename := "unit-target_full_2026-09-22T02-00-00Z.sql.gz"
	if result.Filename != wantFilename {
		t.Errorf("Filename = %q, want %q", result.Filename, wantFilename)
	}
	stored, ok := st.stored[wantFilename]
	if !ok {
		t.Fatalf("expected %q to be stored, got keys: %v", wantFilename, keysOf(st.stored))
	}
	if len(stored) == 0 {
		t.Error("expected non-empty stored (gzip-compressed) bytes")
	}
}

func TestRun_EncryptsWhenRequested(t *testing.T) {
	driver.Register("stub-run-2", func(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
		return &stubConnector{}, &stubBackuper{payload: []byte("secret dump bytes")}, nil
	})
	identity, _ := age.GenerateX25519Identity()

	st := &stubStorage{}
	result, err := Run(context.Background(), Options{
		TargetName:  "enc-target",
		Engine:      "stub-run-2",
		DumpFileExt: "sql",
		Compress:    "none",
		Encrypt:     true,
		Recipients:  []string{identity.Recipient().String()},
		Storage:     st,
		Now:         time.Now,
	}, discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !bytes.HasSuffix([]byte(result.Filename), []byte(".sql.age")) {
		t.Errorf("Filename = %q, want a .sql.age suffix", result.Filename)
	}
	stored := st.stored[result.Filename]
	if bytes.Contains(stored, []byte("secret dump bytes")) {
		t.Error("expected stored bytes to be encrypted, found plaintext")
	}
}

func TestRun_ConnectionFailureIsTaggedByStage(t *testing.T) {
	driver.Register("stub-run-3", func(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
		return &stubConnector{pingErr: errors.New("connection refused")}, &stubBackuper{}, nil
	})

	_, err := Run(context.Background(), Options{
		TargetName: "fail-target",
		Engine:     "stub-run-3",
		Storage:    &stubStorage{},
		Now:        time.Now,
	}, discardLogger())
	if err == nil {
		t.Fatal("expected an error")
	}
	var be *Error
	if !errors.As(err, &be) {
		t.Fatalf("expected a *backupjob.Error, got %T: %v", err, err)
	}
	if be.Stage != "test-connection" {
		t.Errorf("Stage = %q, want %q", be.Stage, "test-connection")
	}
}

func keysOf(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
