package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nweber23/dbtote/internal/driver"
)

func seedDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO widgets (name) VALUES ('a'), ('b'), ('c')"); err != nil {
		t.Fatalf("seed rows: %v", err)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "source.db")
	seedDB(t, srcPath)

	_, backuper, _ := New(driver.ConnectionConfig{Database: srcPath})
	var dump bytes.Buffer
	if _, err := backuper.Backup(context.Background(), driver.BackupOptions{Database: srcPath, Output: &dump}); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if dump.Len() == 0 {
		t.Fatal("expected a non-empty backup file")
	}

	destPath := filepath.Join(t.TempDir(), "restored.db")
	_, _, restorer := New(driver.ConnectionConfig{Database: destPath})
	if err := restorer.Restore(context.Background(), driver.RestoreOptions{Database: destPath, Input: bytes.NewReader(dump.Bytes())}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	db, err := sql.Open("sqlite", destPath)
	if err != nil {
		t.Fatalf("open restored db: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM widgets").Scan(&count); err != nil {
		t.Fatalf("query restored db: %v", err)
	}
	if count != 3 {
		t.Errorf("row count = %d, want 3", count)
	}
}

func TestBackup_SafeUnderConcurrentWrites(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "source.db")
	seedDB(t, srcPath)

	writerDB, err := sql.Open("sqlite", srcPath)
	if err != nil {
		t.Fatalf("open writer db: %v", err)
	}
	defer writerDB.Close()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = writerDB.Exec("INSERT INTO widgets (name) VALUES ('concurrent')")
				time.Sleep(time.Millisecond)
			}
		}
	}()

	_, backuper, _ := New(driver.ConnectionConfig{Database: srcPath})
	var dump bytes.Buffer
	_, err = backuper.Backup(context.Background(), driver.BackupOptions{Database: srcPath, Output: &dump})
	close(stop)
	wg.Wait()

	if err != nil {
		t.Fatalf("Backup under concurrent writes: %v", err)
	}
	if dump.Len() == 0 {
		t.Error("expected a non-empty backup even with concurrent writers")
	}
}

func TestConnector_PingsAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ping.db")
	seedDB(t, path)

	connector, _, _ := New(driver.ConnectionConfig{Database: path})
	if err := connector.Connect(context.Background(), driver.ConnectionConfig{Database: path}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer connector.Close()
	if err := connector.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestCapabilities(t *testing.T) {
	_, backuper, restorer := New(driver.ConnectionConfig{})
	if backuper.SupportsIncremental() {
		t.Error("expected SupportsIncremental() to be false for sqlite")
	}
	if backuper.SupportsDifferential() {
		t.Error("expected SupportsDifferential() to be false for sqlite")
	}
	if backuper.IncrementalBasis() != driver.BasisNone {
		t.Errorf("IncrementalBasis() = %v, want BasisNone", backuper.IncrementalBasis())
	}
	if restorer.SupportsSelective() {
		t.Error("expected SupportsSelective() to be false for sqlite")
	}
}
