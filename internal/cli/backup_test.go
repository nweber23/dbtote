package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/nweber23/dbtote/internal/driver"
)

type stubConn struct{}

func (stubConn) Connect(ctx context.Context, cfg driver.ConnectionConfig) error { return nil }
func (stubConn) Ping(ctx context.Context) error                                 { return nil }
func (stubConn) Close() error                                                   { return nil }

type stubBackuper struct{}

func (stubBackuper) Backup(ctx context.Context, opts driver.BackupOptions) (driver.BackupResult, error) {
	n, err := opts.Output.Write([]byte("-- dump --"))
	return driver.BackupResult{BytesWritten: int64(n)}, err
}
func (stubBackuper) SupportsIncremental() bool                     { return false }
func (stubBackuper) SupportsDifferential() bool                    { return false }
func (stubBackuper) IncrementalBasis() driver.IncrementalBasisKind { return driver.BasisNone }

func TestBackupCommand_RequiresTarget(t *testing.T) {
	driver.Register("cli-test-engine", func(cfg driver.ConnectionConfig) (driver.Connector, driver.Backuper, driver.Restorer) {
		return stubConn{}, stubBackuper{}, nil
	})

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"backup", "--host", "h", "--user", "u", "--database", "d"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error when --target is omitted")
	}
}
