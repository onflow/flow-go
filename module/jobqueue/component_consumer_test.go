package jobqueue

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ComponentConsumerSuite struct {
	suite.Suite

	defaultIndex   uint64
	maxProcessing  uint64
	maxSearchAhead uint64
}

func TestComponentConsumerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ComponentConsumerSuite))
}

func (suite *ComponentConsumerSuite) SetupTest() {
	suite.defaultIndex = uint64(0)
	suite.maxProcessing = uint64(2)
	suite.maxSearchAhead = uint64(5)
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

func generateTestData(jobCount uint64) map[uint64]TestJob {
	jobData := make(map[uint64]TestJob, jobCount)

	for i := uint64(1); i <= jobCount; i++ {
		jobData[i] = TestJob{i}
	}

	return jobData
}

func (suite *ComponentConsumerSuite) prepareTest(
	processor JobProcessor,
	preNotifier NotifyDone,
	postNotifier NotifyDone,
	jobData map[uint64]TestJob,
) (*ComponentConsumer, chan struct{}) {

	jobs := mockJobs(jobData)
	workSignal := make(chan struct{}, 1)

	progress := new(storagemock.ConsumerProgress)
	progress.On("ProcessedIndex").Return(suite.defaultIndex, nil)
	progress.On("SetProcessedIndex", mock.AnythingOfType("uint64")).Return(nil)

	consumer, err := NewComponentConsumer(
		zerolog.New(os.Stdout).With().Timestamp().Logger(),
		workSignal,
		progress,
		jobs,
		suite.defaultIndex,
		processor,
		suite.maxProcessing,
		suite.maxSearchAhead,
	)
	require.NoError(suite.T(), err)
	consumer.SetPreNotifier(preNotifier)
	consumer.SetPostNotifier(postNotifier)

	return consumer, workSignal
}

// TestHappyPath:
// - processes jobs until cancelled
// - notify called for all jobs
func (suite *ComponentConsumerSuite) TestHappyPath() {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	testJobsCount := uint64(20)
	jobData := generateTestData(testJobsCount)
	finishedJobs := make(map[uint64]bool, testJobsCount)

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}

	processor := func(_ irrecoverable.SignalerContext, _ module.Job, complete func()) { complete() }
	notifier := func(jobID module.JobID) {
		defer wg.Done()

		index, err := JobIDToIndex(jobID)
		assert.NoError(suite.T(), err)

		mu.Lock()
		defer mu.Unlock()
		finishedJobs[index] = true

		suite.T().Logf("job %d finished", index)
	}

	suite.Run("runs and notifies using pre-notifier", func() {
		wg.Add(int(testJobsCount))
		consumer, workSignal := suite.prepareTest(processor, nil, notifier, jobData)

		suite.runTest(testCtx, consumer, func() {
			workSignal <- struct{}{}
			wg.Wait()
		})

		// verify all jobs were run
		mu.Lock()
		defer mu.Unlock()
		assert.Len(suite.T(), finishedJobs, len(jobData))
		for index := range jobData {
			assert.True(suite.T(), finishedJobs[index], "job %d did not finished", index)
		}
	})

	suite.Run("runs and notifies using post-notifier", func() {
		wg.Add(int(testJobsCount))
		consumer, workSignal := suite.prepareTest(processor, notifier, nil, jobData)

		suite.runTest(testCtx, consumer, func() {
			workSignal <- struct{}{}
			wg.Wait()
		})

		// verify all jobs were run
		mu.Lock()
		defer mu.Unlock()
		assert.Len(suite.T(), finishedJobs, len(jobData))
		for index := range jobData {
			assert.True(suite.T(), finishedJobs[index], "job %d did not finished", index)
		}
	})
}

// TestProgressesOnComplete:
// - only processes next job after complete is called
func (suite *ComponentConsumerSuite) TestProgressesOnComplete() {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	stopIndex := uint64(10)
	testJobsCount := uint64(11)
	jobData := generateTestData(testJobsCount)
	finishedJobs := make(map[uint64]bool, testJobsCount)

	mu := sync.Mutex{}
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

		mu.Lock()
		defer mu.Unlock()
		finishedJobs[index] = true

		suite.T().Logf("job %d finished", index)
		if index == stopIndex+1 {
			close(done)
		}
	}

	suite.maxProcessing = 1
	consumer, workSignal := suite.prepareTest(processor, nil, notifier, jobData)

	suite.runTest(testCtx, consumer, func() {
		workSignal <- struct{}{}
		unittest.RequireNeverClosedWithin(suite.T(), done, 100*time.Millisecond, fmt.Sprintf("job %d wasn't supposed to finish", stopIndex+1))
	})

	// verify all jobs were run
	mu.Lock()
	defer mu.Unlock()
	assert.Len(suite.T(), finishedJobs, int(stopIndex))
	for index := range finishedJobs {
		assert.LessOrEqual(suite.T(), index, stopIndex)
	}
}

func (suite *ComponentConsumerSuite) TestSignalsBeforeReadyDoNotCheck() {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	suite.defaultIndex = uint64(100)
	started := atomic.NewBool(false)

	jobConsumer := modulemock.NewJobConsumer(suite.T())
	jobConsumer.On("Start").Return(func() error {
		// force Start to take a while so the processingLoop is ready first
		// the processingLoop should wait to start, otherwise Check would be called
		time.Sleep(500 * time.Millisecond)
		started.Store(true)
		return nil
	})
	jobConsumer.On("Stop")

	wg := sync.WaitGroup{}
	wg.Add(1)

	jobConsumer.On("Check").Run(func(_ mock.Arguments) {
		assert.True(suite.T(), started.Load(), "check was called before started")
		wg.Done()
	})

	consumer, workSignal := suite.prepareTest(nil, nil, nil, nil)
	consumer.consumer = jobConsumer

	// send a signal before the component starts to ensure Check would be called if the
	// processingLoop was started
	workSignal <- struct{}{}

	ctx, cancel := context.WithCancel(testCtx)
	signalCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)
	consumer.Start(signalCtx)

	unittest.RequireCloseBefore(suite.T(), consumer.Ready(), 1*time.Second, "timeout waiting for consumer to be ready")
	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 100*time.Millisecond, "check was not called")
	cancel()
	unittest.RequireCloseBefore(suite.T(), consumer.Done(), 1*time.Second, "timeout waiting for consumer to be done")
}

// TestPassesIrrecoverableErrors:
// - throws an irrecoverable error
// - verifies no jobs were processed
func (suite *ComponentConsumerSuite) TestPassesIrrecoverableErrors() {
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

	consumer, _ := suite.prepareTest(processor, nil, notifier, jobData)

	ctx, cancel := context.WithCancel(testCtx)
	signalCtx, errChan := irrecoverable.WithSignaler(ctx)

	consumer.Start(signalCtx)
	unittest.RequireCloseBefore(suite.T(), consumer.Ready(), 100*time.Millisecond, "timeout waiting for consumer to be ready")

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
	unittest.RequireCloseBefore(suite.T(), consumer.Done(), 100*time.Millisecond, "timeout waiting for consumer to be done")

	// no notification should have been sent
	unittest.RequireNotClosed(suite.T(), done, "job wasn't supposed to finish")
}

func (suite *ComponentConsumerSuite) runTest(
	testCtx context.Context,
	consumer *ComponentConsumer,
	sendJobs func(),
) {
	ctx, cancel := context.WithCancel(testCtx)
	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	consumer.Start(signalerCtx)
	unittest.RequireCloseBefore(suite.T(), consumer.Ready(), 100*time.Millisecond, "timeout waiting for the consumer to be ready")

	sendJobs()

	// shutdown
	cancel()
	unittest.RequireCloseBefore(suite.T(), consumer.Done(), 100*time.Millisecond, "timeout waiting for the consumer to be done")
}
