package uploader

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/go-multierror"
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

func (u *GCPBucketUploader) Upload(computationResult *execution.ComputationResult) (err error) {
	var errs *multierror.Error

	objectName := GCPBlockDataObjectName(computationResult)
	object := u.bucket.Object(objectName)

	writer := object.NewWriter(u.ctx)

	defer func() {
		// flush and close the stream
		// this occasionally fails with HTTP 50x errors due to flakiness on the network/GCP API
		closeErr := writer.Close()
		if closeErr != nil {
			errs = multierror.Append(errs, fmt.Errorf("error while closing GCP object: %w", closeErr))
		}

		err = errs.ErrorOrNil()
	}()

	// serialize and write computation result to upload stream
	writeErr := WriteComputationResultsTo(computationResult, writer)
	if writeErr != nil {
		errs = multierror.Append(errs, fmt.Errorf("error while writing computation result to GCP object: %w", writeErr))
		return
	}

	return
}

func GCPBlockDataObjectName(computationResult *execution.ComputationResult) string {
	return fmt.Sprintf("%s.cbor", computationResult.ExecutableBlock.ID().String())
}
