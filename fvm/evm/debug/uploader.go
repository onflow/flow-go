package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/googleapis/gax-go/v2"
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

	client.SetRetry()
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

	// setup writer
	wc := g.bucket.Object(id).Retryer(
		// set retry with the exponential backoff
		storage.WithBackoff(gax.Backoff{
			Initial:    10 * time.Second,
			Max:        uploadTimeout,
			Multiplier: 2,
		})).NewWriter(ctx)

	// write data
	if _, err := wc.Write(data); err != nil {
		return err
	}

	if err := wc.Close(); err != nil {
		return err
	}

	return nil
}
