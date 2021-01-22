package jobqueue_test

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// 0# means job at index 0 is processed.
// +1 means received a job 1
// 1! means job 1 is being processed.
// 1* means job 1 is finished

func TestConsumer(t *testing.T) {
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

	// [+1, +2, +3, ... +12, +13, +14, 1*, 2*, 3*, 5*, 6*, ...12*] => [1#, 2#, 3#, 4!, 5*, 6*, ... 12*, 13, 14]
	// when there are too many pending jobs, it will stop processing more but wait for job 4 to finish
	t.Run("testTooManyPending", testTooManyPending)

	// [+1, +2, +3, +4, Stop, 2*] => [0#, 1!, 2*, 3!, 4]
	// when Stop is called, it won't work on any job any more
	t.Run("testStopRunning", testStopRunning)
}

func testOnStartup(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		assertProcessed(t, cp, 0)
	})
}

// [+1] => 									[0#, 1!]
// when received job 1, it will be processed
func testOnReceiveOneJob(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1

		c.Check()

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1})
		assertProcessed(t, cp, 0)
	})
}

// [+1, 1*] => 							[0#, 1#]
// when job 1 is finished, it will be marked as processed
func testOnJobFinished(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1

		c.Check()
		c.FinishJob(jobIDAtIndex(1))

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1})
		assertProcessed(t, cp, 1)
	})
}

// [+1, +2, 1*, 2*] => 			[0#, 1#, 2#]
// when job 2 and 1 are finished, they will be marked as processed
func testOnJobsFinished(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		c.FinishJob(jobIDAtIndex(1)) // 1*
		c.FinishJob(jobIDAtIndex(2)) // 2*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1, 2})
		assertProcessed(t, cp, 2)
	})
}

// [+1, +2, +3, +4] => 			[0#, 1!, 2!, 3!, 4]
// when more jobs are arrived than the max number of workers, only the first 3 jobs will be processed
func testMaxWorker(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
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

		w.AssertCalled(t, []int{1, 2, 3})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*] => 	[0#, 1!, 2!, 3*, 4!]
// when job 3 is finished, which is not the next processing job 1, the processed index won't change
func testNonNextFinished(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.FinishJob(jobIDAtIndex(3)) // 3*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1, 2, 3, 4})
	})
}

// [+1, +2, +3, +4, 3*, 2*] => 			[0#, 1!, 2*, 3*, 4!]
// when job 3 and 2 are finished, the processed index won't change, because 1 is still not finished
func testTwoNonNextFinished(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.FinishJob(jobIDAtIndex(3)) // 3*
		c.FinishJob(jobIDAtIndex(2)) // 2*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1, 2, 3, 4})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5] =>	[0#, 1!, 2*, 3*, 4!, 5!]
// when job 5 is received, it will be processed, because the worker has capacity
func testProcessingWithNonNextFinished(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.FinishJob(jobIDAtIndex(3)) // 3*
		c.FinishJob(jobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5, +6] =>	[0#, 1!, 2*, 3*, 4!, 5!, 6]
// when job 6 is received, no more worker can process it, it will be buffered
func testMaxWorkerWithFinishedNonNexts(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.FinishJob(jobIDAtIndex(3)) // 3*
		c.FinishJob(jobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		require.NoError(t, j.PushOne()) // +6
		c.Check()

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 0)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5, 1*] => [0#, 1#, 2#, 3#, 4!, 5!]
// when job 1 is finally finished, it will fast forward the processed index to 3
func testFastforward(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.FinishJob(jobIDAtIndex(3)) // 3*
		c.FinishJob(jobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		c.FinishJob(jobIDAtIndex(1)) // 1*

		time.Sleep(1 * time.Millisecond)

		w.AssertCalled(t, []int{1, 2, 3, 4, 5})
		assertProcessed(t, cp, 3)
	})
}

// [+1, +2, +3, +4, 3*, 2*, +5, 1*, +6, +7, 6*], restart => [0#, 1#, 2#, 3#, 4!, 5!, 6*, 7!]
// when job queue crashed and restarted, the queue can be resumed
func testWorkOnNextAfterFastforward(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		require.NoError(t, j.PushOne()) // +2
		c.Check()

		require.NoError(t, j.PushOne()) // +3
		c.Check()

		require.NoError(t, j.PushOne()) // +4
		c.Check()

		c.FinishJob(jobIDAtIndex(3)) // 3*
		c.FinishJob(jobIDAtIndex(2)) // 2*

		require.NoError(t, j.PushOne()) // +5
		c.Check()

		c.FinishJob(jobIDAtIndex(1)) // 1*

		require.NoError(t, j.PushOne()) // +6
		c.Check()

		require.NoError(t, j.PushOne()) // +7
		c.Check()

		c.FinishJob(jobIDAtIndex(6)) // 6*

		time.Sleep(1 * time.Millisecond)

		// rebuild a consumer with the dependencies to simulate a restart
		// jobs need to be reused, since it stores all the jobs
		reWorker := newMockWorker()
		reProgress := badger.NewChunkConsumer(db)
		reConsumer := newTestConsumer(reProgress, j, reWorker)

		err := reConsumer.Start()
		require.NoError(t, err)

		time.Sleep(1 * time.Millisecond)

		reWorker.AssertCalled(t, []int{4, 5, 6})
		assertProcessed(t, reProgress, 3)
	})
}

// [+1, +2, +3, ... +12 , 1*, 2*, 3*, 5*, 6*, ...12*, +13, +14] => [1#, 2#, 3#, 4!, 5*, 6*, ... 12*, 13, 14]
// when there are too many pending (8) jobs, it will stop processing more but wait for job 4 to finish
// 5* - 12* has 8 pending in total
func testTooManyPending(t *testing.T) {
	runWith(t, func(c *testConsumer, cp storage.ConsumerProgress, w *mockWorker, j *mockJobs, db *badgerdb.DB) {
		require.NoError(t, c.Start())
		for i := 1; i <= 12; i++ {
			// +1, +2, ... +12
			require.NoError(t, j.PushOne())
			c.Check()
		}

		for i := 1; i <= 12; i++ {
			// job 1 - 12 are all finished, except 4
			// 5, 6, 7, 8, 9, 10, 11, 12 are pending, which have reached max pending (8)
			if i == 4 {
				continue
			}
			c.FinishJob(jobIDAtIndex(i))
		}

		require.NoError(t, j.PushOne()) // +13
		c.Check()

		require.NoError(t, j.PushOne()) // + 14
		c.Check()

		time.Sleep(1 * time.Millisecond)

		// max pending reached, will not work on job 13
		w.AssertCalled(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
		assertProcessed(t, cp, 3)
	})
}

func testStopRunning(t *testing.T) {
}

type JobStatus = jobqueue.JobStatus
type Worker = jobqueue.Worker
type JobID = storage.JobID
type Job = storage.Job

func runWith(t *testing.T, runTestWith func(*testConsumer, storage.ConsumerProgress, *mockWorker, *mockJobs, *badgerdb.DB)) {
	unittest.RunWithBadgerDB(t, func(db *badgerdb.DB) {
		jobs := newMockJobs()
		worker := newMockWorker()
		progress := badger.NewChunkConsumer(db)
		consumer := newTestConsumer(progress, jobs, worker)
		runTestWith(consumer, progress, worker, jobs, db)
	})
}

func assertProcessed(t *testing.T, cp storage.ConsumerProgress, expectProcessed int) {
	processed, err := cp.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, expectProcessed, processed)
}

// testConsumer wraps the Consumer instance, and allows
// us to inspect its internal state
type testConsumer struct {
	*jobqueue.Consumer
}

// func (c *testConsumer) ReadState() (bool, int, map[int]*JobStatus, map[JobID]int) {
// 	return c.running, c.processedIndex, c.processings, c.processingsIndex
// }

func newTestConsumer(cp storage.ConsumerProgress, jobs storage.Jobs, worker Worker) *testConsumer {
	log := unittest.Logger().With().Str("module", "consumer").Logger()
	maxProcessing := 3
	maxPending := 8
	c := jobqueue.NewConsumer(log, jobs, cp, worker, maxProcessing, maxPending)
	return &testConsumer{
		Consumer: c,
	}
}

// mockJobs implements the Jobs interface, and is used as the dependency for
// the Consumer for testing purpose
type mockJobs struct {
	sync.Mutex
	log      zerolog.Logger
	last     int
	jobs     map[int]Job
	index    map[JobID]int
	jobMaker *jobMaker
}

func newMockJobs() *mockJobs {
	return &mockJobs{
		log:      unittest.Logger().With().Str("module", "jobs").Logger(),
		last:     0, // must be from 1
		jobs:     make(map[int]Job),
		index:    make(map[JobID]int),
		jobMaker: newJobMaker(),
	}
}

// var _ storage.Jobs = &mockJobs{}

func (j *mockJobs) AtIndex(index int) (Job, error) {
	j.Lock()
	defer j.Unlock()

	job, ok := j.jobs[index]

	j.log.Debug().Int("index", index).Bool("exists", ok).Msg("reading job at index")

	if !ok {
		return nil, storage.ErrNotFound
	}

	return job, nil
}

func (j *mockJobs) Add(job Job) error {
	j.Lock()
	defer j.Unlock()

	j.log.Debug().Str("job_id", string(job.ID())).Msg("adding job")

	id := job.ID()
	_, ok := j.index[id]
	if ok {
		return storage.ErrAlreadyExists
	}

	index := j.last + 1
	j.index[id] = index
	j.jobs[index] = job
	j.last++

	j.log.
		Debug().Str("job_id", string(job.ID())).
		Int("index", index).
		Msg("job added at index")

	return nil
}

func (j *mockJobs) PushOne() error {
	job := j.jobMaker.Next()
	return j.Add(job)
}

func (j *mockJobs) PushN(n int) error {
	for i := 0; i < n; i++ {
		err := j.PushOne()
		if err != nil {
			return err
		}
	}
	return nil
}

// deterministically compute the JobID from index
func jobIDAtIndex(index int) storage.JobID {
	return storage.JobID(fmt.Sprintf("%v", index))
}

// a Mock worker that stores all the jobs that it was asked to work on
type mockWorker struct {
	sync.Mutex
	log    zerolog.Logger
	called []Job
}

// var _ module.Worker = &mockWorker{}

func newMockWorker() *mockWorker {
	return &mockWorker{
		log:    unittest.Logger().With().Str("module", "worker").Logger(),
		called: make([]Job, 0),
	}
}

func (w *mockWorker) Run(job Job) {
	w.Lock()
	defer w.Unlock()

	w.log.Debug().Str("job_id", string(job.ID())).Msg("worker called with job")

	w.called = append(w.called, job)
}

// return the IDs of the jobs
func (w *mockWorker) AssertCalled(t *testing.T, expectCalled []int) {
	called := make([]int, 0)
	for _, c := range w.called {
		jobID, err := strconv.Atoi(string(c.ID()))
		require.NoError(t, err)
		called = append(called, jobID)
	}
	sort.Ints(called)
	require.Equal(t, expectCalled, called)
}

type testJob struct {
	index int
}

func (tj testJob) ID() storage.JobID {
	return jobIDAtIndex(tj.index)
}

// jobMaker is a test helper.
// it creates new job with unique job id
type jobMaker struct {
	sync.Mutex
	index int
}

func newJobMaker() *jobMaker {
	return &jobMaker{
		index: 1,
	}
}

// return next unique job
func (j *jobMaker) Next() Job {
	j.Lock()
	defer j.Unlock()

	job := &testJob{
		index: j.index,
	}
	j.index++
	return job
}
