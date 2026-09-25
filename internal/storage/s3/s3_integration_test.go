//go:build integration

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestS3Backend_StoreRetrieveListDelete_AgainstS3Mock(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "adobe/s3mock:latest",
		ExposedPorts: []string{"9090/tcp"},
		WaitingFor:   wait.ForListeningPort("9090/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start s3mock container: %v", err)
	}
	defer container.Terminate(ctx)

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9090/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	backend, err := New("dbtote-test-bucket", "us-east-1", "dbtote/", endpoint)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := backend.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &backend.bucket}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	if err := backend.Store(ctx, "target_full_2026-09-22T02-00-00Z.sql.gz", bytes.NewBufferString("dump contents")); err != nil {
		t.Fatalf("Store: %v", err)
	}

	r, err := backend.Retrieve(ctx, "target_full_2026-09-22T02-00-00Z.sql.gz")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if string(got) != "dump contents" {
		t.Errorf("got %q, want %q", got, "dump contents")
	}

	metas, err := backend.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 || metas[0].Name != "target_full_2026-09-22T02-00-00Z.sql.gz" {
		t.Errorf("unexpected List result: %+v", metas)
	}

	if err := backend.Delete(ctx, "target_full_2026-09-22T02-00-00Z.sql.gz"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := backend.Retrieve(ctx, "target_full_2026-09-22T02-00-00Z.sql.gz"); err == nil {
		t.Error("expected Retrieve to fail after Delete")
	}
}
