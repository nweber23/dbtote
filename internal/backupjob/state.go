package backupjob

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type BackupRecord struct {
	Filename     string
	Target       string
	Type         string
	Storage      string
	Size         int64
	Timestamp    time.Time
	BasisPointer string
}

type StateDB struct {
	db *sql.DB
}

func OpenStateDB(path string) (*StateDB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("backupjob: create state db directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("backupjob: open state db: %w", err)
	}
	const schema = `
CREATE TABLE IF NOT EXISTS backups (
	filename      TEXT PRIMARY KEY,
	target        TEXT NOT NULL,
	type          TEXT NOT NULL,
	storage       TEXT NOT NULL,
	size          INTEGER NOT NULL,
	timestamp     TEXT NOT NULL,
	basis_pointer TEXT NOT NULL DEFAULT ''
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("backupjob: create schema: %w", err)
	}
	return &StateDB{db: db}, nil
}

func (s *StateDB) Close() error { return s.db.Close() }

func (s *StateDB) RecordBackup(ctx context.Context, rec BackupRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO backups (filename, target, type, storage, size, timestamp, basis_pointer) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.Filename, rec.Target, rec.Type, rec.Storage, rec.Size, rec.Timestamp.UTC().Format(time.RFC3339), rec.BasisPointer,
	)
	if err != nil {
		return fmt.Errorf("backupjob: record backup %q: %w", rec.Filename, err)
	}
	return nil
}

func (s *StateDB) DeleteBackup(ctx context.Context, filename string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM backups WHERE filename = ?`, filename); err != nil {
		return fmt.Errorf("backupjob: delete backup %q: %w", filename, err)
	}
	return nil
}

type ListFilter struct {
	Target  string
	Storage string
	Since   time.Time
}

func (s *StateDB) ListBackups(ctx context.Context, filter ListFilter) ([]BackupRecord, error) {
	query := "SELECT filename, target, type, storage, size, timestamp, basis_pointer FROM backups WHERE 1=1"
	var args []any
	if filter.Target != "" {
		query += " AND target = ?"
		args = append(args, filter.Target)
	}
	if filter.Storage != "" {
		query += " AND storage = ?"
		args = append(args, filter.Storage)
	}
	if !filter.Since.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.Since.UTC().Format(time.RFC3339))
	}
	query += " ORDER BY timestamp ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("backupjob: list backups: %w", err)
	}
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var rec BackupRecord
		var ts string
		if err := rows.Scan(&rec.Filename, &rec.Target, &rec.Type, &rec.Storage, &rec.Size, &ts, &rec.BasisPointer); err != nil {
			return nil, fmt.Errorf("backupjob: scan backup row: %w", err)
		}
		rec.Timestamp, _ = time.Parse(time.RFC3339, ts)
		records = append(records, rec)
	}
	return records, rows.Err()
}
