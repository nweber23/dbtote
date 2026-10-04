//go:build integration

package mongodb

import (
	"bytes"
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"

	"github.com/nweber23/dbtote/internal/driver"
)

func TestMongoBackupRestoreRoundTrip_RealServer(t *testing.T) {
	ctx := context.Background()

	container, err := tcmongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongo container: %v", err)
	}
	defer container.Terminate(ctx)

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	seedClient, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("seed connect: %v", err)
	}
	defer seedClient.Disconnect(ctx)
	coll := seedClient.Database("dbtote_it").Collection("widgets")
	if _, err := coll.InsertMany(ctx, []any{
		bson.D{{Key: "name", Value: "a"}},
		bson.D{{Key: "name", Value: "b"}},
		bson.D{{Key: "name", Value: "c"}},
	}); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	cfg := driver.ConnectionConfig{Database: "dbtote_it", Extra: map[string]string{"uri": uri}}

	_, backuper, _ := New(cfg)
	var dump bytes.Buffer
	if _, err := backuper.Backup(ctx, driver.BackupOptions{Database: cfg.Database, Output: &dump}); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if dump.Len() == 0 {
		t.Fatal("expected a non-empty archive")
	}

	if err := coll.Drop(ctx); err != nil {
		t.Fatalf("drop collection to simulate loss: %v", err)
	}

	_, _, restorer := New(cfg)
	if err := restorer.Restore(ctx, driver.RestoreOptions{Database: cfg.Database, Input: bytes.NewReader(dump.Bytes())}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	count, err := coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}
