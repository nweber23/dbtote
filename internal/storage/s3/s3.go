package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/nweber23/dbtote/internal/storage"
)

func init() {
	storage.Register("s3", func(cfg map[string]string) (storage.Backend, error) {
		return New(cfg["bucket"], cfg["region"], cfg["prefix"], "")
	})
}

type Backend struct {
	bucket string
	prefix string
	client *s3.Client
}

func New(bucket, region, prefix, endpointOverride string) (*Backend, error) {
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) { o.MaxAttempts = 3 })
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpointOverride != "" {
			o.BaseEndpoint = aws.String(endpointOverride)
			o.UsePathStyle = true
		}
	})
	return &Backend{bucket: bucket, prefix: prefix, client: client}, nil
}

func (b *Backend) key(name string) string {
	if b.prefix == "" {
		return name
	}
	return strings.TrimSuffix(b.prefix, "/") + "/" + name
}

func (b *Backend) Store(ctx context.Context, name string, r io.Reader) error {
	uploader := transfermanager.New(b.client)
	key := b.key(name)
	if _, err := uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
		Body:   r,
	}); err != nil {
		return fmt.Errorf("s3: upload %q: %w", name, err)
	}
	return nil
}

func (b *Backend) Retrieve(ctx context.Context, name string) (io.ReadCloser, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.bucket), Key: aws.String(b.key(name))})
	if err != nil {
		return nil, fmt.Errorf("s3: get %q: %w", name, err)
	}
	return out.Body, nil
}

func (b *Backend) List(ctx context.Context, prefix string) ([]storage.BackupMeta, error) {
	fullPrefix := b.key(prefix)
	var metas []storage.BackupMeta
	paginator := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(fullPrefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3: list: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			name := strings.TrimPrefix(strings.TrimPrefix(*obj.Key, b.prefix), "/")
			meta := storage.BackupMeta{Name: name}
			if obj.Size != nil {
				meta.Size = *obj.Size
			}
			if obj.LastModified != nil {
				meta.Timestamp = *obj.LastModified
			}
			metas = append(metas, meta)
		}
	}
	return metas, nil
}

func (b *Backend) Delete(ctx context.Context, name string) error {
	if _, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(b.bucket), Key: aws.String(b.key(name))}); err != nil {
		return fmt.Errorf("s3: delete %q: %w", name, err)
	}
	return nil
}
