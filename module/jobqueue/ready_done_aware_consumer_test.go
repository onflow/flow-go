package jobqueue

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ReadyDoneAwareConsumerSuite struct {
	suite.Suite

	db *badger.DB

	defaultIndex   uint64
	maxProcessing  uint64
	maxSearchAhead uint64

	progress *storagemock.ConsumerProgress
}

func TestReadyDoneAwareConsumerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ReadyDoneAwareConsumerSuite))
}

func (suite *ReadyDoneAwareConsumerSuite) SetupTest() {
	suite.defaultIndex = uint64(0)
	suite.maxProcessing = uint64(2)
	suite.maxSearchAhead = uint64(5)

	suite.progress = new(storagemock.ConsumerProgress)
}

func mockJobs(data map[uint64]TestJob) *modulemock.Jobs {
	jobs := new(modulemock.Jobs)

	jobs.On("AtIndex", mock.AnythingOfType("uint64")).Return(
		func(index uint64) module.Job {
			job, ok := data[index]
			if !ok {
				return nil
			}
			return job
		},
		func(index uint64) error {
			_, ok := data[index]
			if !ok {
				return storage.ErrNotFound
			}
			return nil
		},
	)

	return jobs
}

func mockProgress() *storagemock.ConsumerProgress {
	progress := new(storagemock.ConsumerProgress)
	progress.On("ProcessedIndex").Return(uint64(0), nil)
	progress.On("SetProcessedIndex", mock.AnythingOfType("uint64")).Return(nil)

	return progress
}

func generateTestData(jobCount uint64) map[uint64]TestJob {
	jobData := make(map[uint64]TestJob, jobCount)

	for i := uint64(1); i <= jobCount; i++ {
		jobData[i] = TestJob{i}
	}

	return jobData
}

func (suite *ReadyDoneAwareConsumerSuite) prepareTest(
	processor JobProcessor,
	notifier NotifyDone,
	jobData map[uint64]TestJob,
) (*ReadyDoneAwareConsumer, chan struct{}) {

	jobs := mockJobs(jobData)
	workSignal := make(chan struct{})
	progress := mockProgress()

	consumer, err := NewReadyDoneAwareConsumer(
		zerolog.New(os.Stdout).With().Timestamp().Logger(),
		progress,
		jobs,
		processor,
		workSignal,
		suite.defaultIndex,
		suite.maxProcessing,
		suite.maxSearchAhead,
		notifier,
	)
	assert.NoError(suite.T(), err)

	return consumer, workSignal
}

// TestHappyPath:
// - processes jobs until cancelled
// - notify called for all jobs
func (suite *ReadyDoneAwareConsumerSuite) TestHappyPath() {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	testJobsCount := uint64(20)
	jobData := generateTestData(testJobsCount)
	finishedJobs := make(map[uint64]bool, testJobsCount)

	wg := sync.WaitGroup{}
	wg.Add(int(testJobsCount))

	processor := func(_ irrecoverable.SignalerContext, _ module.Job, complete func()) { complete() }
	notifier := func(jobID module.JobID) {
		defer wg.Done()

		index, err := JobIDToIndex(jobID)
		assert.NoError(suite.T(), err)

		finishedJobs[index] = true

		suite.T().Logf("job %d finished", index)
	}

	consumer, workSignal := suite.prepareTest(processor, notifier, jobData)

	suite.runTest(testCtx, consumer, workSignal, func() {
		workSignal <- struct{}{}
		wg.Wait()
	})

	// verify all jobs were run
	assert.Len(suite.T(), finishedJobs, len(jobData))
	for index := range jobData {
		assert.True(suite.T(), finishedJobs[index], "job %d did not finished", index)
	}
}

// TestProgressesOnComplete:
// - only processes next job after complete is called
func (suite *ReadyDoneAwareConsumerSuite) TestProgressesOnComplete() {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	stopIndex := uint64(10)
	testJobsCount := uint64(11)
	jobData := generateTestData(testJobsCount)
	finishedJobs := make(map[uint64]bool, testJobsCount)

	done := make(chan struct{})

	processor := func(_ irrecoverable.SignalerContext, job module.Job, complete func()) {
		index, err := JobIDToIndex(job.ID())
		assert.NoError(suite.T(), err)

		if index <= stopIndex {
			complete()
		}
	}
	notifier := func(jobID module.JobID) {
		index, err := JobIDToIndex(jobID)
		assert.NoError(suite.T(), err)

		finishedJobs[index] = true

		suite.T().Logf("job %d finished", index)
		if index == stopIndex+1 {
			close(done)
		}
	}

	suite.maxProcessing = 1
	consumer, workSignal := suite.prepareTest(processor, notifier, jobData)

	suite.runTest(testCtx, consumer, workSignal, func() {
		workSignal <- struct{}{}
		unittest.RequireNeverClosedWithin(suite.T(), done, 50*time.Millisecond, fmt.Sprintf("job %d wasn't supposed to finish", stopIndex+1))
	})

	// verify all jobs were run
	assert.Len(suite.T(), finishedJobs, int(stopIndex))
	for index := range finishedJobs {
		assert.LessOrEqual(suite.T(), index, stopIndex)
	}
}

// TestPassesIrrecoverableErrors:
// - throws an irrecoverable error
// - verifies no jobs were processed
func (suite *ReadyDoneAwareConsumerSuite) TestPassesIrrecoverableErrors() {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	testJobsCount := uint64(10)
	jobData := generateTestData(testJobsCount)
	done := make(chan struct{})

	expectedErr := fmt.Errorf("test failure")

	// always throws an error
	processor := func(ctx irrecoverable.SignalerContext, job module.Job, _ func()) {
		ctx.Throw(expectedErr)
	}

	// never expects a job
	notifier := func(jobID module.JobID) {
		suite.T().Logf("job %s finished unexpectedly", jobID)
		close(done)
	}

	consumer, _ := suite.prepareTest(processor, notifier, jobData)

	ctx, cancel := context.WithCancel(testCtx)
	signalCtx, errChan := irrecoverable.WithSignaler(ctx)

	consumer.Start(signalCtx)
	unittest.RequireCloseBefore(suite.T(), consumer.Ready(), 10*time.Millisecond, "timeout waiting for consumer to be ready")

	// send job signal, then wait for the irrecoverable error
	// don't need to sent signal since the worker is kicked off by Start()
	select {
	case <-ctx.Done():
		suite.T().Errorf("expected irrecoverable error, but got none")
	case err := <-errChan:
		assert.ErrorIs(suite.T(), err, expectedErr)
	}

	// shutdown
	cancel()
	unittest.RequireCloseBefore(suite.T(), consumer.Done(), 10*time.Millisecond, "timeout waiting for consumer to be done")

	// no notification should have been sent
	unittest.RequireNotClosed(suite.T(), done, "job wasn't supposed to finish")
}

func (suite *ReadyDoneAwareConsumerSuite) runTest(
	testCtx context.Context,
	consumer *ReadyDoneAwareConsumer,
	workSignal chan<- struct{},
	sendJobs func(),
) {
	ctx, cancel := context.WithCancel(testCtx)
	signalCtx, errChan := irrecoverable.WithSignaler(ctx)

	// use global context so we listen for errors until the test is finished
	go irrecoverableNotExpected(suite.T(), testCtx, errChan)

	consumer.Start(signalCtx)
	unittest.RequireCloseBefore(suite.T(), consumer.Ready(), 10*time.Millisecond, "timeout waiting for consumer to be ready")

	sendJobs()

	// shutdown
	cancel()
	unittest.RequireCloseBefore(suite.T(), consumer.Done(), 10*time.Millisecond, "timeout waiting for consumer to be done")
}

func irrecoverableNotExpected(t *testing.T, ctx context.Context, errChan <-chan error) {
	select {
	case <-ctx.Done():
		return
	case err := <-errChan:
		require.NoError(t, err, "unexpected irrecoverable error")
	}
}
