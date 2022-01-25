package uploader

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/module/metrics"
	testutils "github.com/onflow/flow-go/utils/unittest"
)

func Test_AsyncUploader(t *testing.T) {

	computationResult := unittest.ComputationResultFixture(nil)

	t.Run("uploads are run in parallel and emit metrics", func(t *testing.T) {
		wgCalled := sync.WaitGroup{}
		wgCalled.Add(3)

		wgAllDone := sync.WaitGroup{}
		wgAllDone.Add(1)

		uploader := &DummyUploader{
			f: func() error {
				// this should be called 3 times
				wgCalled.Done()

				wgAllDone.Wait()

				return nil
			},
		}

		metrics := &DummyCollector{}
		async := NewAsyncUploader(uploader, 1*time.Nanosecond, 1, zerolog.Nop(), metrics)

		err := async.Upload(computationResult)
		require.NoError(t, err)

		err = async.Upload(computationResult)
		require.NoError(t, err)

		err = async.Upload(computationResult)
		require.NoError(t, err)

		wgCalled.Wait() // all three are in progress, check metrics

		require.Equal(t, int64(3), metrics.Counter.Load())

		wgAllDone.Done() //release all

		<-async.Done()

		require.Equal(t, int64(0), metrics.Counter.Load())
		require.True(t, metrics.DurationTotal.Load() > 0, "duration should be nonzero")
	})

	t.Run("failed uploads are retried", func(t *testing.T) {

		callCount := 0

		wg := sync.WaitGroup{}
		wg.Add(1)

		uploader := &DummyUploader{
			f: func() error {
				if callCount < 3 {
					callCount++
					return fmt.Errorf("artificial upload error")
				}
				wg.Done()
				return nil
			},
		}

		async := NewAsyncUploader(uploader, 1*time.Nanosecond, 5, zerolog.Nop(), &metrics.NoopCollector{})

		err := async.Upload(computationResult)
		require.NoError(t, err)

		wg.Wait()

		require.Equal(t, 3, callCount)
	})

	time.Sleep(1 * time.Second)

	t.Run("stopping component stops retrying", func(t *testing.T) {
		testutils.SkipUnless(t, testutils.TEST_FLAKY, "flaky")

		callCount := 0

		wg := sync.WaitGroup{}
		wg.Add(1)

		uploader := &DummyUploader{
			f: func() error {
				defer func() {
					callCount++
				}()
				wg.Wait()
				return fmt.Errorf("this should return only once")
			},
		}

		async := NewAsyncUploader(uploader, 1*time.Nanosecond, 5, zerolog.Nop(), &metrics.NoopCollector{})

		err := async.Upload(computationResult) // doesn't matter what we upload
		require.NoError(t, err)

		c := async.Done()
		wg.Done()
		<-c

		require.Equal(t, 1, callCount)
	})

}

func Test_GCPBucketUploader(t *testing.T) {
	testutils.SkipUnless(t, testutils.TEST_REQUIRES_GCP_ACCESS, "requires GCP Bucket setup")

	bucketName := os.Getenv("FLOW_TEST_GCP_BUCKET_NAME")
	if bucketName == "" {
		t.Fatal("please set FLOW_TEST_GCP_BUCKET_NAME environmental variable")
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
	f func() error
}

func (d *DummyUploader) Upload(_ *execution.ComputationResult) error {
	return d.f()
}

type FailingUploader struct {
	failTimes int
	callCount int
}

func (d *FailingUploader) Upload(_ *execution.ComputationResult) error {
	defer func() {
		d.callCount++
	}()

	if d.callCount <= d.failTimes {
		return fmt.Errorf("an artificial error")
	}

	return nil
}

type DummyCollector struct {
	metrics.NoopCollector
	Counter       atomic.Int64
	DurationTotal atomic.Int64
}

func (d *DummyCollector) ExecutionBlockDataUploadStarted() {
	d.Counter.Inc()
}

func (d *DummyCollector) ExecutionBlockDataUploadFinished(dur time.Duration) {
	d.Counter.Dec()
	d.DurationTotal.Add(dur.Nanoseconds())
}
