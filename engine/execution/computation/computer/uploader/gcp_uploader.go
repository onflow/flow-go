package uploader

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
)

type GCPBucketUploader struct {
	log    zerolog.Logger
	bucket *storage.BucketHandle
	ctx    context.Context
}

func NewGCPBucketUploader(ctx context.Context, bucketName string, log zerolog.Logger) (*GCPBucketUploader, error) {

	// no need to close the client according to documentation
	// https://pkg.go.dev/cloud.google.com/go/storage#Client.Close
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

	return &GCPBucketUploader{
		bucket: bucket,
		log:    log.With().Str("subcomponent", "gcp_bucket_uploader").Logger(),
		ctx:    ctx,
	}, nil
}

func (u *GCPBucketUploader) Upload(computationResult *execution.ComputationResult) error {

	objectName := GCPBlockDataObjectName(computationResult)
	object := u.bucket.Object(objectName)

	writer := object.NewWriter(u.ctx)
	defer func() {
		err := writer.Close()
		if err != nil {
			u.log.Warn().Err(err).Str("object_name", objectName).Msg("error while closing GCP object")
		}
	}()

	return WriteComputationResultsTo(computationResult, writer)
}

func GCPBlockDataObjectName(computationResult *execution.ComputationResult) string {
	return fmt.Sprintf("%s.cbor", computationResult.ExecutableBlock.ID().String())
}
