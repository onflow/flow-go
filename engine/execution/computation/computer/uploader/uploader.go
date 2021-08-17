package uploader

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"cloud.google.com/go/storage"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/module"
	"github.com/rs/zerolog"
)

type Uploader interface {
	Upload(computationResult *execution.ComputationResult) error
}

func NewAsyncUploader(uploader Uploader, log zerolog.Logger, metrics module.ExecutionMetrics) *AsyncUploader {
	return &AsyncUploader{
		unit:     engine.NewUnit(),
		uploader: uploader,
		log:      log.With().Str("component", "block_data_uploader").Logger(),
		metrics:  metrics,
	}
}

type AsyncUploader struct {
	unit     *engine.Unit
	uploader Uploader
	log      zerolog.Logger
	metrics  module.ExecutionMetrics
}

func (a *AsyncUploader) Ready() <-chan struct{} {
	return a.unit.Ready()
}

func (a *AsyncUploader) Done() <-chan struct{} {
	return a.unit.Done()
}

func (a *AsyncUploader) Upload(computationResult *execution.ComputationResult) error {
	a.unit.Launch(func() {
		a.metrics.ExecutionBlockDataUploadStarted()
		start := time.Now()

		err := a.uploader.Upload(computationResult)
		if err != nil {
			a.log.Error().Err(err).Msg("error while uploading block data")
		}

		a.metrics.ExecutionBlockDataUploadFinished(time.Since(start))
	})
	return nil
}

func NewGCPBucketUploader(ctx context.Context, bucketName string, log zerolog.Logger) (*GCPBucketUploader, error) {

	// no need to close the client according to documentation
	// https://pkg.go.dev/cloud.google.com/go/storage#Client.Close
	ctx = context.Background()
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

type GCPBucketUploader struct {
	log    zerolog.Logger
	bucket *storage.BucketHandle
	ctx    context.Context
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

func NewFileUploader(dir string) *FileUploader {
	return &FileUploader{
		dir: dir,
	}
}

type FileUploader struct {
	dir string
}

func (f *FileUploader) Upload(computationResult *execution.ComputationResult) error {
	file, err := os.Create(path.Join(f.dir, fmt.Sprintf("%s.cbor", computationResult.ExecutableBlock.ID())))
	if err != nil {
		return fmt.Errorf("cannot create file for writing block data: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return WriteComputationResultsTo(computationResult, writer)
}
