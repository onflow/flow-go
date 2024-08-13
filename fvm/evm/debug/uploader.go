package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
)

type Uploader interface {
	Upload(id string, data json.RawMessage) error
}

var _ Uploader = &GCPUploader{}

type GCPUploader struct {
	bucket *storage.BucketHandle
}

func NewGCPUploader(bucketName string) (*GCPUploader, error) {
	// no need to close the client according to documentation
	// https://pkg.go.dev/cloud.google.com/go/storage#Client.Close
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create GCP Bucket client: %w", err)
	}
	bucket := client.Bucket(bucketName)

	// try accessing buckets to validate settings
	_, err = bucket.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while listing bucket attributes: %w", err)
	}

	return &GCPUploader{
		bucket: bucket,
	}, nil
}

// Upload traces to the GCP for the given id.
// The upload will be retried by the client if the error is transient.
func (g *GCPUploader) Upload(id string, data json.RawMessage) error {
	const uploadTimeout = time.Minute * 5
	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()

	wc := g.bucket.Object(id).NewWriter(ctx)
	if _, err := wc.Write(data); err != nil {
		return err
	}

	if err := wc.Close(); err != nil {
		return err
	}

	return nil
}

type NoopUploader struct{}

func (np *NoopUploader) Upload(id string, data json.RawMessage) error {
	return nil
}
