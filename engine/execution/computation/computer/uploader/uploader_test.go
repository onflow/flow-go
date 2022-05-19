package uploader

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/debug"
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
		wgUploadStarted := sync.WaitGroup{}
		wgUploadStarted.Add(3)

		wgContinueUpload := sync.WaitGroup{}
		wgContinueUpload.Add(1)

		uploader := &DummyUploader{
			f: func() error {
				// this should be called 3 times
				wgUploadStarted.Done()

				wgContinueUpload.Wait()

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

		wgUploadStarted.Wait() // all three are in progress, check metrics

		require.Equal(t, int64(3), metrics.Counter.Load())

		wgContinueUpload.Done() //release all

		// shut down component
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
				// force an upload error to test that upload is retried 3 times
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

	// This test shuts down the async uploader right after the upload has started. The upload has an error to force
	// the retry mechanism to kick in (under normal circumstances). Since the component is shutting down, the retry
	// should not kick in.
	//
	// sequence of events:
	// 1. create async uploader and initiate upload with an error - to force retrying
	// 2. shut down async uploader right after upload initiated (not completed)
	// 3. assert that upload called only once
	t.Run("stopping component stops retrying", func(t *testing.T) {
		testutils.SkipUnless(t, testutils.TEST_FLAKY, "flaky")

		callCount := 0
		t.Log("test started grID:", string(bytes.Fields(debug.Stack())[1]))

		// this wait group ensures that async uploader has a chance to start the upload before component is shut down
		// otherwise, there's a race condition that can happen where the component can shut down before the async uploader
		// has a chance to start the upload
		wgUploadStarted := sync.WaitGroup{}
		wgUploadStarted.Add(1)

		// this wait group ensures that async uploader won't send an error (to test if retry will kick in) until
		// the component has initiated shutting down (which should stop retry from working)
		wgShutdownStarted := sync.WaitGroup{}
		wgShutdownStarted.Add(1)
		t.Log("added 1 to wait group grID:", string(bytes.Fields(debug.Stack())[1]))

		uploader := &DummyUploader{
			f: func() error {
				t.Log("DummyUploader func() start - incrementing callCount grID:", string(bytes.Fields(debug.Stack())[1]))
				callCount++

				t.Log("DummyUploader func() - about to call wgUploadStarted.Done() grID:", string(bytes.Fields(debug.Stack())[1]))
				// signal to main goroutine that upload started, so it can initiate shutting down component
				wgUploadStarted.Done()
				//func() {
				//	callCount++
				//}()
				t.Log("DummyUpload func() waiting for component shutdown to start grID:", string(bytes.Fields(debug.Stack())[1]))
				wgShutdownStarted.Wait()
				t.Log("DummyUploader func() component shutdown started, about to return error grID:", string(bytes.Fields(debug.Stack())[1]))
				return fmt.Errorf("this should return only once")
			},
		}
		t.Log("about to create NewAsyncUploader grID:", string(bytes.Fields(debug.Stack())[1]))
		async := NewAsyncUploader(uploader, 1*time.Nanosecond, 5, zerolog.Nop(), &metrics.NoopCollector{})
		t.Log("about to call async.Upload() grID:", string(bytes.Fields(debug.Stack())[1]))
		err := async.Upload(computationResult) // doesn't matter what we upload
		require.NoError(t, err)

		// stop component and check that it's fully stopped
		t.Log("about to close async uploader grID:", string(bytes.Fields(debug.Stack())[1]))

		// wait until upload has started before shutting down the component
		wgUploadStarted.Wait()

		// import unittest2 "github.com/onflow/flow-go/utils/unittest"
		//unittest2.RequireCloseBefore(t, async.Done(), 5*time.Second, "async uploader not closed in time")
		// stop component and check that it's fully stopped
		t.Log("about to initiate shutdown grID: ", string(bytes.Fields(debug.Stack())[1]))
		c := async.Done()
		t.Log("about to notify upload() that shutdown started and can continue uploading grID:", string(bytes.Fields(debug.Stack())[1]))
		wgShutdownStarted.Done()
		t.Log("about to read from async.Done() channel to see if it's closed grID:", string(bytes.Fields(debug.Stack())[1]))
		_, ok := <-c
		t.Log("about to check if async.Done() channel is closed grID:", string(bytes.Fields(debug.Stack())[1]))
		require.False(t, ok, "component Done channel not closed")

		t.Log("about to check if callCount is 1 grID:", string(bytes.Fields(debug.Stack())[1]))
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
