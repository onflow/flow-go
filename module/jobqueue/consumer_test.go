package jobqueue

import (
	"sync"
	"testing"

	badgerdb "github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// 0# means job at index 0 is processed.
// +1 means received a job 1
// 1! means job 1 is being processed.
// 1* means job 1 is finished, but there is a gap in the processed

type testConsumer struct {
	*Consumer
}

func (c *testConsumer) readState() (bool, int, map[int]*JobStatus, map[JobID]int) {
	return c.running, c.processedIndex, c.processings, c.processingsIndex
}

func NewTestConsumer(db *badgerdb.DB, fn func(Job)) *testConsumer {
	log := unittest.Logger()
	jobs := &MockJobs{}
	cc := badger.NewChunkConsumer(db)
	maxProcessing := 3
	maxPending := 8
	c := NewConsumer(log, jobs, cc, maxProcessing, maxPending, fn)
	return &testConsumer{
		Consumer: c,
	}
}

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
	// when job 6 is received, then no more worker can process it, it will be buffered
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
}

func testOnReceiveOneJob(t *testing.T) {
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

type MockJobs struct {
	sync.Mutex
	last  int
	jobs  map[int]Job
	index map[JobID]int
}

// ensure MockJobs implements the Jobs interface
var _ storage.Jobs = &MockJobs{}

func (j *MockJobs) AtIndex(index int) (Job, error) {
	j.Lock()
	defer j.Unlock()

	job, ok := j.jobs[index]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return job, nil
}

func (j *MockJobs) Add(job Job) error {
	j.Lock()
	defer j.Unlock()

	id := job.ID()
	_, ok := j.index[id]
	if ok {
		return storage.ErrAlreadyExists
	}

	index := j.last + 1
	j.index[id] = index
	j.jobs[index] = job
	j.last++

	return nil
}
