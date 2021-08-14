package uploader

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func Test_AsyncUploader(t *testing.T) {

	wgCalled := sync.WaitGroup{}
	wgCalled.Add(3)

	wgAllDone := sync.WaitGroup{}
	wgAllDone.Add(1)

	uploader := &DummyUploader{
		f: func() {
			// this should be called 3 times
			wgCalled.Done()

			wgAllDone.Wait()
		},
	}

	metrics := &DummyCollector{}
	async := NewAsyncUploader(uploader, zerolog.Nop(), metrics)

	err := async.Upload(nil)
	require.NoError(t, err)

	err = async.Upload(nil)
	require.NoError(t, err)

	err = async.Upload(nil)
	require.NoError(t, err)

	wgCalled.Wait() // all three are in progress, check metrics

	require.Equal(t, int64(3), metrics.Counter.Load())

	wgAllDone.Done() //release all

	<-async.Done()

	require.Equal(t, int64(0), metrics.Counter.Load())
}

func Test_GCPBucketUploader(t *testing.T) {

	t.Skip("requires GCP Bucket setup")

	bucketName := os.Getenv("FLOW_TEST_GCP_BUCKET")
	if bucketName == "" {
		t.Fatal("please set FLOW_TEST_GCP_BUCKET environmental variable")
	}
	uploader, err := NewGCPBucketUploader(context.Background(), bucketName, zerolog.Nop())
	require.NoError(t, err)

	cr := generateComputationResult(t)

	buffer := &bytes.Buffer{}
	err = WriteComputationResultsTo(cr, buffer)
	require.NoError(t, err)

	err = uploader.Upload(cr)

	require.NoError(t, err)

	// check uploaded object
	client, err := storage.NewClient(context.Background())
	require.NoError(t, err)
	bucket := client.Bucket(bucketName)

	objectName := GCPBlockDataObjectName(cr)

	reader, err := bucket.Object(objectName).NewReader(context.Background())
	require.NoError(t, err)

	readBytes, err := ioutil.ReadAll(reader)
	require.NoError(t, err)

	require.Equal(t, buffer.Bytes(), readBytes)
}

type DummyUploader struct {
	f func()
}

func (d *DummyUploader) Upload(_ *execution.ComputationResult) error {
	d.f()
	return nil
}

type DummyCollector struct {
	metrics.NoopCollector
	Counter atomic.Int64
}

func (d *DummyCollector) ExecutionBlockDataUploadStarted() {
	d.Counter.Inc()
}

func (d *DummyCollector) ExecutionBlockDataUploadFinished() {
	d.Counter.Dec()
}
