package uploader

import (
	"bytes"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/execution"
	exeunittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_AsyncUploader(t *testing.T) {

	computationResult := exeunittest.ComputationResultFixture(
		t,
		unittest.IdentifierFixture(),
		nil)

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
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
		async.Start(ctx)

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
		cancel()
		unittest.AssertClosesBefore(t, async.Done(), 1*time.Second, "async uploader did not finish in time")
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
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
		async.Start(ctx)
		defer cancel()

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
	// 3. assert that upload called only once even when trying to use retry mechanism
	t.Run("stopping component stops retrying", func(t *testing.T) {
		unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky")

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
				t.Log("DummyUploader func() - about to call wgUploadStarted.Done() grID:", string(bytes.Fields(debug.Stack())[1]))
				// signal to main goroutine that upload started, so it can initiate shutting down component
				wgUploadStarted.Done()

				t.Log("DummyUpload func() waiting for component shutdown to start grID:", string(bytes.Fields(debug.Stack())[1]))
				wgShutdownStarted.Wait()
				t.Log("DummyUploader func() component shutdown started, about to return error grID:", string(bytes.Fields(debug.Stack())[1]))

				// force an upload error to test that upload is never retried (because component is shut down)
				// normally, we would see retry mechanism kick in and the callCount would be > 1
				// but since component has started shutting down, we expect callCount to be 1
				// In summary, callCount SHOULD be called only once - but we want the test to TRY and call it more than once to prove that it
				// was only called it once. If we changed it to 'callCount < 1' that wouldn't prove that the test tried to call it more than once
				// and wouldn't prove that stopping the component stopped the retry mechanism.
				if callCount < 5 {
					t.Logf("DummyUploader func() incrementing callCount=%d grID: %s", callCount, string(bytes.Fields(debug.Stack())[1]))
					callCount++
					t.Logf("DummyUploader func() about to return error callCount=%d grID: %s", callCount, string(bytes.Fields(debug.Stack())[1]))
					return fmt.Errorf("this should return only once")
				}
				return nil
			},
		}
		t.Log("about to create NewAsyncUploader grID:", string(bytes.Fields(debug.Stack())[1]))
		async := NewAsyncUploader(uploader, 1*time.Nanosecond, 5, zerolog.Nop(), &metrics.NoopCollector{})
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
		async.Start(ctx)
		t.Log("about to call async.Upload() grID:", string(bytes.Fields(debug.Stack())[1]))
		err := async.Upload(computationResult) // doesn't matter what we upload
		require.NoError(t, err)

		// stop component and check that it's fully stopped
		t.Log("about to close async uploader grID:", string(bytes.Fields(debug.Stack())[1]))

		// wait until upload has started before shutting down the component
		wgUploadStarted.Wait()

		// stop component and check that it's fully stopped
		t.Log("about to initiate shutdown grID: ", string(bytes.Fields(debug.Stack())[1]))
		cancel()
		t.Log("about to notify upload() that shutdown started and can continue uploading grID:", string(bytes.Fields(debug.Stack())[1]))
		wgShutdownStarted.Done()
		t.Log("about to check async done channel is closed grID:", string(bytes.Fields(debug.Stack())[1]))
		unittest.RequireCloseBefore(t, async.Done(), 1*time.Second, "async uploader not closed in time")

		t.Log("about to check if callCount is 1 grID:", string(bytes.Fields(debug.Stack())[1]))
		require.Equal(t, 1, callCount)
	})

	t.Run("onComplete callback called if set", func(t *testing.T) {
		var onCompleteCallbackCalled = false

		wgUploadCalleded := sync.WaitGroup{}
		wgUploadCalleded.Add(1)

		uploader := &DummyUploader{
			f: func() error {
				wgUploadCalleded.Done()
				return nil
			},
		}

		async := NewAsyncUploader(uploader, 1*time.Nanosecond, 1, zerolog.Nop(), &DummyCollector{})
		async.SetOnCompleteCallback(func(computationResult *execution.ComputationResult, err error) {
			onCompleteCallbackCalled = true
		})
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
		async.Start(ctx)

		err := async.Upload(computationResult)
		require.NoError(t, err)

		wgUploadCalleded.Wait()
		cancel()
		unittest.AssertClosesBefore(t, async.Done(), 1*time.Second, "async uploader not done in time")

		require.True(t, onCompleteCallbackCalled)
	})
}

// DummyUploader is an Uploader implementation with an Upload() callback
type DummyUploader struct {
	f func() error
}

func (d *DummyUploader) Upload(_ *execution.ComputationResult) error {
	return d.f()
}

// FailingUploader mocks upload failure cases
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

// DummyCollector is test uploader metrics implementation
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
