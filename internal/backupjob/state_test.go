package backupjob

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStateDB_RecordAndList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := OpenStateDB(path)
	if err != nil {
		t.Fatalf("OpenStateDB: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	older := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	if err := db.RecordBackup(ctx, BackupRecord{Filename: "a", Target: "t1", Type: "full", Storage: "local-main", Size: 10, Timestamp: older}); err != nil {
		t.Fatalf("RecordBackup a: %v", err)
	}
	if err := db.RecordBackup(ctx, BackupRecord{Filename: "b", Target: "t1", Type: "full", Storage: "local-main", Size: 20, Timestamp: newer}); err != nil {
		t.Fatalf("RecordBackup b: %v", err)
	}
	if err := db.RecordBackup(ctx, BackupRecord{Filename: "c", Target: "t2", Type: "full", Storage: "local-main", Size: 30, Timestamp: newer}); err != nil {
		t.Fatalf("RecordBackup c: %v", err)
	}

	records, err := db.ListBackups(ctx, ListFilter{Target: "t1"})
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(records), records)
	}
	if records[0].Filename != "a" || records[1].Filename != "b" {
		t.Errorf("expected ascending timestamp order a, b; got %+v", records)
	}

	if err := db.DeleteBackup(ctx, "a"); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	records, _ = db.ListBackups(ctx, ListFilter{Target: "t1"})
	if len(records) != 1 || records[0].Filename != "b" {
		t.Errorf("expected only b to remain, got %+v", records)
	}
}

func TestStateDB_ListSinceFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, _ := OpenStateDB(path)
	defer db.Close()
	ctx := context.Background()

	if err := db.RecordBackup(ctx, BackupRecord{Filename: "old", Target: "t", Timestamp: time.Now().Add(-72 * time.Hour)}); err != nil {
		t.Fatalf("RecordBackup old: %v", err)
	}
	if err := db.RecordBackup(ctx, BackupRecord{Filename: "recent", Target: "t", Timestamp: time.Now().Add(-1 * time.Hour)}); err != nil {
		t.Fatalf("RecordBackup recent: %v", err)
	}

	records, err := db.ListBackups(ctx, ListFilter{Target: "t", Since: time.Now().Add(-24 * time.Hour)})
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(records) != 1 || records[0].Filename != "recent" {
		t.Errorf("expected only the recent record, got %+v", records)
	}
}
