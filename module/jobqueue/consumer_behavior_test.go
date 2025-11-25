package jobqueue_test

import (
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	DefaultIndex = uint64(0)
	ConsumerTag  = "consumer"
)

// 0# means job at index 0 is processed.
// +1 means received a job 1
// 1! means job 1 is being processed.
// 1* means job 1 is finished

func TestConsumer(t *testing.T) {
	t.Parallel()

	// [] => 										[0#]
	// on startup, if there is no more job, nothing is processed
	t.Run("testOnStartup", testOnStartup)

	// [+1] => 									[0#, 1!]
	// when received job 1, it will be processed
	t.Run("testOnReceiveOneJob", testOnReceiveOneJob)

	// [+1, 1*] => 							[0#, 1#]
	// when job 1 is finished, it will be marked as processed
	t.Run("testOnJobFinished", testOnJobFinished)

	// [+1, +2, 1*, 2*] => 			[0#, 1#, 2#]
	// when job 2 and 1 are finished, they will be marked as processed
	t.Run("testOnJobsFinished", testOnJobsFinished)

	// [+1, +2, +3, +4] => 			[0#, 1!, 2!, 3!, 4]
	// when more jobs are arrived than the max number of workers, only the first 3 jobs will be processed
	t.Run("testMaxWorker", testMaxWorker)

	// [+1, +2, +3, +4, +5, +6] => [0#, !1, *2, *3, *4, *5, 6, +7] => [0#, *1, *2, *3, *4, *5, !6, !7]
	// when processing lags behind, the consumer is paused until processing catches up
	t.Run("testPauseResume", testPauseResume)

	// [+1, +2, +3, +4, 3*] => 	[0#, 1!, 2!, 3*, 4!]
	// when job 3 is finished, which is not the next processing job 1, the processed index won't change
	t.Run("testNonNextFinished", testNonNextFinished)

	// [+1, +2, +3, +4, 3*, 2*] => 			[0#, 1!, 2*, 3*, 4!]
	// when job 3 and 2 are finished, the processed index won't change, because 1 is still not finished
	t.Run("testTwoNonNextFinished", testTwoNonNextFinished)

	// [+1, +2, +3, +4, 3*, 2*, +5] =>	[0#, 1!, 2*, 3*, 4!, 5!]
	// when job 5 is received, it will be processed, because the worker has capacity
	t.Run("testProcessingWithNonNextFinished", testProcessingWithNonNextFinished)

	// [+1, +2, +3, +4, 3*, 2*, +5, +6] =>	[0#, 1!, 2*, 3*, 4!, 5!, 6]
	// when job 6 is received, no more worker can process it, it will be buffered
	t.Run("testMaxWorkerWithFinishedNonNexts", testMaxWorkerWithFinishedNonNexts)

	// [+1, +2, +3, +4, 3*, 2*, +5, 1*] => [0#, 1#, 2#, 3#, 4!, 5!]
	// when job 1 is finally finished, it will fast forward the processed index to 3
	t.Run("testFastforward", testFastforward)

	// [+1, +2, +3, +4, 3*, 2*, +5, 1*, +6, +7, 6*], restart => [0#, 1#, 2#, 3#, 4!, 5!, 6*, 7!]
	// when job queue crashed and restarted, the queue can be resumed
	t.Run("testWorkOnNextAfterFastforward", testWorkOnNextAfterFastforward)

	t.Run("testMovingProcessedIndex", testMovingProcessedIndex)

	// [+1, +2, +3, +4, Stop, 2*] => [0#, 1!, 2*, 3!, 4]
	// when Stop is called, it won't work on any job any more
	t.Run("testStopRunning", testStopRunning)

	t.Run("testConcurrency", testConcurrency)
}

func testOnStartup(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		assertProcessed(t, cp, 0)
	})
}

func TestProcessedOrder(t *testing.T) {
	runWith(t, 5, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		assertProcessed(t, cp, 5)
	})
}

// [+1] => 									[0#, 1!]
// when received job 1, it will be processed
func testOnReceiveOneJob(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1

		c.Check()

		time.Sleep(100 * time.Millisecond)

		w.AssertCalled(t, []int64{1})
		assertProcessed(t, cp, 0)
	})
}

// [+1, 1*] => 							[0#, 1#]
// when job 1 is finished, it will be marked as processed
func testOnJobFinished(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1

		c.Check()
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1))

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1})
		assertProcessed(t, cp, 1)
	})
}

// [+1, +2, 1*, 2*] => 			[0#, 1#, 2#]
// when job 2 and 1 are finished, they will be marked as processed
func testOnJobsFinished(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1)) // 1*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2})
		assertProcessed(t, cp, 2)
	})
}

// [+1, +2, +3, +4] => 			[0#, 1!, 2!, 3!, 4]
// when more jobs are arrived than the max number of workers, only the first 3 jobs will be processed
func testMaxWorker(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2, 3})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, +5, +6] => [0#, !1, *2, *3, *4, *5, 6, +7] => [0#, *1, *2, *3, *4, *5, !6, !7]
// when processing lags behind, the consumer is paused until processing catches up
func testPauseResume(t *testing.T) {
	runWithSeatchAhead(t, 5, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		require.NoError(t, j.PushOne()) // +5
		c.Check()
		time.Sleep(1 * time.Millisecond)
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(4)) // 4*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(5)) // 5*

		require.NoError(t, j.PushOne()) // +6
		c.Check()

		time.Sleep(1 * time.Millisecond)

		// all jobs so far are processed, except 1 and 6
		w.AssertCalled(t, []int64{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 0)

		require.NoError(t, j.PushOne())             // +7
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1)) // 1*
		c.Check()

		time.Sleep(1 * time.Millisecond)

		// processing resumed after job 1 finished
		w.AssertCalled(t, []int64{1, 2, 3, 4, 5, 6, 7})
		assertProcessed(t, cp, 5)
	})
}

// [+1, +2, +3, +4, 3*] => 	[0#, 1!, 2!, 3*, 4!]
// when job 3 is finished, which is not the next processing job 1, the processed index won't change
func testNonNextFinished(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2, 3, 4})
	})
}

// testMovingProcessedIndex evaluates that the processedIndex only moves to job at index i when all job indices preceding that are
// already processed.
// +: added job to consumer
// *: finished job
// #: processed job
//
// [+1, +2, +3, +3, +4] => 	[1, 2, 3*, 4] => [1, 2, 3*, 4*] => => [1#, 2, 3*, 4*] => [1#, 2#, 3#, 4#]
func testMovingProcessedIndex(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		// when job 3 is done, the processed index should not move, since all jobs
		// preceding it are not processed.
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		time.Sleep(1 * time.Millisecond)
		processedIndex, err := cp.ProcessedIndex()
		require.NoError(t, err)
		require.Equal(t, processedIndex, uint64(0))

		// when job 4 is done, the processed index should not move, since all jobs
		// preceding it are not processed.
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(4)) // 4*
		time.Sleep(1 * time.Millisecond)
		processedIndex, err = cp.ProcessedIndex()
		require.NoError(t, err)
		require.Equal(t, processedIndex, uint64(0))

		// when job 1 is done, the processed index should move to 1, since all jobs
		// preceding it are processed.
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1)) // 1* -> 1#
		time.Sleep(1 * time.Millisecond)
		processedIndex, err = cp.ProcessedIndex()
		require.NoError(t, err)
		require.Equal(t, processedIndex, uint64(1))

		// when job 2 is done, the processed index should move to 4, since all jobs
		// preceding it are processed.
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2* -> 4#
		time.Sleep(1 * time.Millisecond)
		processedIndex, err = cp.ProcessedIndex()
		require.NoError(t, err)
		require.Equal(t, processedIndex, uint64(4))

		w.AssertCalled(t, []int64{1, 2, 3, 4})
	})
}

// [+1, +2, +3, +4, 3*, 2*] => 			[0#, 1!, 2*, 3*, 4!]
// when job 3 and 2 are finished, the processed index won't change, because 1 is still not finished
func testTwoNonNextFinished(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2, 3, 4})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5] =>	[0#, 1!, 2*, 3*, 4!, 5!]
// when job 5 is received, it will be processed, because the worker has capacity
func testProcessingWithNonNextFinished(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5, +6] =>	[0#, 1!, 2*, 3*, 4!, 5!, 6]
// when job 6 is received, no more worker can process it, it will be buffered
func testMaxWorkerWithFinishedNonNexts(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		require.NoError(t, j.PushOne()) // +6
		c.Check()

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5, 1*] => [0#, 1#, 2#, 3#, 4!, 5!]
// when job 1 is finally finished, it will fast forward the processed index to 3
func testFastforward(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1)) // 1*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int64{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 3)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5, 1*, +6, +7, 6*], restart => [0#, 1#, 2#, 3#, 4!, 5!, 6*, 7!]
// when job queue crashed and restarted, the queue can be resumed
func testWorkOnNextAfterFastforward(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(3)) // 3*
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1)) // 1*

		require.NoError(t, j.PushOne()) // +6
		c.Check()

		require.NoError(t, j.PushOne()) // +7
		c.Check()

		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(6)) // 6*

		time.Sleep(1 * time.Millisecond)

		// rebuild a consumer with the dependencies to simulate a restart
		// jobs need to be reused, since it stores all the jobs
		reWorker := newMockWorker()
		reProgress := store.NewConsumerProgress(pebbleimpl.ToDB(db), ConsumerTag)
		reConsumer := newTestConsumer(t, reProgress, j, reWorker, 0, DefaultIndex)

		progress, err := reProgress.Initialize(DefaultIndex)
		require.NoError(t, err)

		err = reConsumer.Start()
		require.NoError(t, err)

		time.Sleep(1 * time.Millisecond)

		reWorker.AssertCalled(t, []int64{4, 5, 6})
		assertProcessed(t, progress, 3)
	})
}

// [+1, +2, +3, +4, Stop, 2*] => [0#, 1!, 2*, 3!, 4]
// when Stop is called, it won't work on any job any more
func testStopRunning(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		for i := 0; i < 4; i++ {
			require.NoError(t, j.PushOne())
			c.Check()
		}

		// graceful shutdown and wait for goroutines that
		// are calling worker.Run to finish
		c.Stop()

		// it won't work on 4 because it stopped before 2 is finished
		w.AssertCalled(t, []int64{1, 2, 3})
		assertProcessed(t, cp, 0)

		// still allow the existing job to finish
		c.NotifyJobIsDone(jobqueue.JobIDAtIndex(1))
		assertProcessed(t, cp, 1)
	})
}

func testConcurrency(t *testing.T) {
	runWith(t, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		require.NoError(t, c.Start())
		var finishAll sync.WaitGroup
		finishAll.Add(100)
		// Finish job concurrently
		w.fn = func(j Job) {
			go func() {
				c.NotifyJobIsDone(j.ID())
				finishAll.Done()
			}()
		}

		// Pushing job and checking job concurrently
		var pushAll sync.WaitGroup
		for i := 0; i < 100; i++ {
			pushAll.Add(1)
			go func() {
				require.NoError(t, j.PushOne())
				c.Check()
				pushAll.Done()
			}()
		}

		// wait until pushed all
		pushAll.Wait()

		// wait until finished all
		finishAll.Wait()

		called := make([]int64, 0)
		for i := 1; i <= 100; i++ {
			called = append(called, int64(i))
		}

		w.AssertCalled(t, called)
		assertProcessed(t, cp, 100)
	})
}

type JobID = module.JobID
type Job = module.Job

func runWith(t testing.TB,
	defaultIndex uint64,
	runTestWith func(module.JobConsumer, storage.ConsumerProgress, *mockWorker, *jobqueue.MockJobs, *pebble.DB)) {
	runWithSeatchAhead(t, 0, defaultIndex, runTestWith)
}

func runWithSeatchAhead(t testing.TB, maxSearchAhead uint64, defaultIndex uint64,
	runTestWith func(module.JobConsumer, storage.ConsumerProgress, *mockWorker, *jobqueue.MockJobs, *pebble.DB)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		jobs := jobqueue.NewMockJobs()
		worker := newMockWorker()
		progressInitializer := store.NewConsumerProgress(pebbleimpl.ToDB(pdb), ConsumerTag)
		consumer := newTestConsumer(t, progressInitializer, jobs, worker, maxSearchAhead, defaultIndex)
		progress, err := progressInitializer.Initialize(defaultIndex)
		require.NoError(t, err)
		runTestWith(consumer, progress, worker, jobs, pdb)
	})
}

func assertProcessed(t testing.TB, cp storage.ConsumerProgress, expectProcessed uint64) {
	processed, err := cp.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, expectProcessed, processed)
}

func newTestConsumer(t testing.TB, cp storage.ConsumerProgressInitializer, jobs module.Jobs, worker jobqueue.Worker, maxSearchAhead uint64, defaultIndex uint64) module.JobConsumer {
	log := unittest.Logger().With().Str("module", "consumer").Logger()
	maxProcessing := uint64(3)
	c, err := jobqueue.NewConsumer(log, jobs, cp, worker, maxProcessing, maxSearchAhead, defaultIndex)
	require.NoError(t, err)
	return c
}

// a Mock worker that stores all the jobs that it was asked to work on
type mockWorker struct {
	sync.RWMutex
	log    zerolog.Logger
	called []Job
	fn     func(job Job)
}

func newMockWorker() *mockWorker {
	return &mockWorker{
		log:    unittest.Logger().With().Str("module", "worker").Logger(),
		called: make([]Job, 0),
		fn:     func(Job) {},
	}
}

func (w *mockWorker) Run(job Job) error {
	w.Lock()
	defer w.Unlock()

	w.log.Debug().Str("job_id", string(job.ID())).Msg("worker called with job")

	w.called = append(w.called, job)
	w.fn(job)

	return nil
}

// return the IDs of the jobs
func (w *mockWorker) AssertCalled(t *testing.T, expectCalled []int64) {
	called := make([]int, 0)
	w.RLock()
	for _, c := range w.called {
		jobID, err := strconv.Atoi(string(c.ID()))
		require.NoError(t, err)
		called = append(called, jobID)
	}
	w.RUnlock()
	sort.Ints(called)

	called64 := make([]int64, 0)
	for _, c := range called {
		called64 = append(called64, int64(c))
	}
	require.Equal(t, expectCalled, called64)
}

// if a job can be finished as soon as it's consumed,
// benchmark to see how fast it consume jobs.
// the latest result is
// 0.16 ms to push job
// 0.22 ms to finish job
func BenchmarkPushAndConsume(b *testing.B) {
	b.StopTimer()
	runWith(b, DefaultIndex, func(c module.JobConsumer, cp storage.ConsumerProgress, w *mockWorker, j *jobqueue.MockJobs, db *pebble.DB) {
		var wg sync.WaitGroup
		wg.Add(b.N)

		// Finish job as soon as it's consumed
		w.fn = func(j Job) {
			go func() {
				c.NotifyJobIsDone(j.ID())
				wg.Done()
			}()
		}

		require.NoError(b, c.Start())

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			err := j.PushOne()
			if err != nil {
				b.Error(err)
			}
			c.Check()
		}
		wg.Wait()
		b.StopTimer()
	})
}
