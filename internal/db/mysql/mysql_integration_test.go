//go:build integration

package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"

	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/nweber23/dbtote/internal/driver"
)

func TestMySQLBackupRestoreRoundTrip_RealServer(t *testing.T) {
	ctx := context.Background()

	container, err := tcmysql.Run(ctx, "mysql:8",
		tcmysql.WithDatabase("dbtote_it"),
		tcmysql.WithUsername("root"),
		tcmysql.WithPassword("test-password"),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}
	defer container.Terminate(ctx)

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	cfg := driver.ConnectionConfig{
		Host:     host,
		Port:     int(port.Num()),
		User:     "root",
		Password: "test-password",
		Database: "dbtote_it",
	}

	// Seed a table directly via database/sql so the test has a known
	// row count to verify after restore.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open seed connection: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE widgets (id INT PRIMARY KEY, name VARCHAR(255))"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO widgets VALUES (1, 'a'), (2, 'b'), (3, 'c')"); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	_, backuper, _ := New(cfg)
	var dump bytes.Buffer
	if _, err := backuper.Backup(ctx, driver.BackupOptions{Database: cfg.Database, Output: &dump}); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if dump.Len() == 0 {
		t.Fatal("expected non-empty dump")
	}

	// Simulate data loss, then restore from the dump.
	if _, err := db.Exec("DROP TABLE widgets"); err != nil {
		t.Fatalf("drop table to simulate loss: %v", err)
	}

	_, _, restorer := New(cfg)
	if err := restorer.Restore(ctx, driver.RestoreOptions{Database: cfg.Database, Input: bytes.NewReader(dump.Bytes())}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM widgets").Scan(&count); err != nil {
		t.Fatalf("verify row count: %v", err)
	}
	if count != 3 {
		t.Errorf("row count after restore = %d, want 3", count)
	}
}
