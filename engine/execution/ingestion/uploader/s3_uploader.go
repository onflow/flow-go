package uploader

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
)

var _ Uploader = (*S3Uploader)(nil)

// S3Uploader is a S3 implementation of the uploader interface.
type S3Uploader struct {
	ctx    context.Context
	log    zerolog.Logger
	client *s3.Client
	bucket string
}

// NewS3Uploader returns a new S3 uploader instance.
func NewS3Uploader(ctx context.Context, client *s3.Client, bucket string, log zerolog.Logger) *S3Uploader {
	return &S3Uploader{
		ctx:    ctx,
		log:    log,
		client: client,
		bucket: bucket,
	}
}

// Upload uploads the given computation result to the configured S3 bucket.
func (u *S3Uploader) Upload(result *execution.ComputationResult) error {
	uploader := manager.NewUploader(u.client)
	key := GCPBlockDataObjectName(result)
	buf := &bytes.Buffer{}
	err := WriteComputationResultsTo(result, buf)

	if err != nil {
		return err
	}

	_, err = uploader.Upload(u.ctx, &s3.PutObjectInput{
		Bucket: &u.bucket,
		Key:    &key,
		Body:   buf,
	})

	return err
}
