package jobqueue_test

import (
	"fmt"
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
	runWith(t, func(c *testConsumer, w *mockWorker, j *mockJobs) {
		// running, processedIndex, processings, processingsIndex := c.ReadState()
		// require.Equal(t, running, false)
		// require.Equal(t, processedIndex, 0)
		// require.Equal(t, len(processings), 0)
		// require.Equal(t, len(processingsIndex), 0)
	})
}

// [+1] => 									[0#, 1!]
// when received job 1, it will be processed
func testOnReceiveOneJob(t *testing.T) {
	runWith(t, func(c *testConsumer, w *mockWorker, j *mockJobs) {
		require.NoError(t, c.Start())
		require.NoError(t, j.PushOne()) // +1
		c.Check()

		time.Sleep(1 * time.Second)

		called := []string{"1"}
		require.Equal(t, called, w.Called())
	})
}

func testOnJobFinished(t *testing.T) {
}

func testOnJobsFinished(t *testing.T) {
}

func testMaxWorker(t *testing.T) {
}

func testNonNextFinished(t *testing.T) {
}

func testTwoNonNextFinished(t *testing.T) {
}

func testProcessingWithNonNextFinished(t *testing.T) {
}

func testMaxWorkerWithFinishedNonNexts(t *testing.T) {
}

func testFastforward(t *testing.T) {
}

func testWorkOnNextAfterFastforward(t *testing.T) {
}

func testTooManyPending(t *testing.T) {
}

func testStopRunning(t *testing.T) {
}

type JobStatus = jobqueue.JobStatus
type Worker = jobqueue.Worker
type JobID = storage.JobID
type Job = storage.Job

func runWith(t *testing.T, runTestWith func(*testConsumer, *mockWorker, *mockJobs)) {
	unittest.RunWithBadgerDB(t, func(db *badgerdb.DB) {
		jobs := newMockJobs()
		worker := newMockWorker()
		consumer := newTestConsumer(db, jobs, worker)
		runTestWith(consumer, worker, jobs)
	})
}

// testConsumer wraps the Consumer instance, and allows
// us to inspect its internal state
type testConsumer struct {
	*jobqueue.Consumer
}

// func (c *testConsumer) ReadState() (bool, int, map[int]*JobStatus, map[JobID]int) {
// 	return c.running, c.processedIndex, c.processings, c.processingsIndex
// }

func newTestConsumer(db *badgerdb.DB, jobs storage.Jobs, worker Worker) *testConsumer {
	log := unittest.Logger().With().Str("module", "consumer").Logger()
	cc := badger.NewChunkConsumer(db)
	maxProcessing := 3
	maxPending := 8
	c := jobqueue.NewConsumer(log, jobs, cc, worker, maxProcessing, maxPending)
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
func (w *mockWorker) Called() []string {
	called := make([]string, 0)
	for _, c := range w.called {
		called = append(called, string(c.ID()))
	}
	return called
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
