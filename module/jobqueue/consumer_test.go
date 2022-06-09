package jobqueue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProcessableJobs(t *testing.T) {
	t.Parallel()

	maxProcessing := uint64(3)

	t.Run("no job, nothing to process", func(t *testing.T) {
		jobs := NewMockJobs() // no job in the queue
		processings := map[uint64]*jobStatus{}
		processedIndex := uint64(0)

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(0), processedTo)
		assertJobs(t, []uint64{}, jobsToRun)
	})

	t.Run("max processing was not reached", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[uint64]*jobStatus{}
		for i := uint64(3); i <= 11; i++ {
			// job 3 are 5 are not done, 2 processing in total
			// 4, 6, 7, 8, 9, 10, 11 are finished, 7 finished in total
			done := true
			if i == 3 || i == 5 {
				done = false
			}
			processings[i] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := uint64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(2), processedTo)
		// it will process on more job, and reach the max processing.
		assertJobs(t, []uint64{
			12,
		}, jobsToRun)
	})

	t.Run("reached max processing", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[uint64]*jobStatus{}
		for i := uint64(3); i <= 12; i++ {
			// job 3, 5, 6 are not done, which have reached max processing(3)
			// 4, 7, 8, 9, 10, 11, 12 are finished, 7 finished in total
			done := true
			if i == 3 || i == 5 || i == 6 {
				done = false
			}
			processings[i] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}
		processedIndex := uint64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(2), processedTo)
		// it will not process any job, because the max processing is reached.
		assertJobs(t, []uint64{}, jobsToRun)
	})

	t.Run("no more job", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(11)) // 11 jobs, no more job to process
		processings := map[uint64]*jobStatus{}
		for i := uint64(3); i <= 11; i++ {
			// job 3, 11 are not done, which have not reached max processing (3)
			// 4, 5, 6, 7, 8, 9, 10 are finished, 7 finished in total
			done := true
			if i == 3 || i == 11 {
				done = false
			}
			processings[i] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := uint64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(2), processedTo)
		assertJobs(t, []uint64{}, jobsToRun)
	})

	t.Run("next jobs were done", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[uint64]*jobStatus{}
		for i := uint64(3); i <= 6; i++ {
			// job 3, 5 are done
			// job 4, 6 are not done, which have not reached max processing
			done := true
			if i == 4 || i == 6 {
				done = false
			}
			processings[i] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := uint64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(3), processedTo)
		assertJobs(t, []uint64{
			7,
		}, jobsToRun)
	})

}

// Test after jobs have been processed, the job status are removed to prevent from memory-leak
func TestProcessedIndexDeletion(t *testing.T) {
	setup := func(t *testing.T, f func(c *Consumer, jobs *MockJobs)) {
		unittest.RunWithBadgerDB(t, func(db *badgerdb.DB) {
			log := unittest.Logger().With().Str("module", "consumer").Logger()
			jobs := NewMockJobs()
			progress := badger.NewConsumerProgress(db, "consumer")
			worker := newMockWorker()
			maxProcessing := uint64(3)
			c := NewConsumer(log, jobs, progress, worker, maxProcessing)
			worker.WithConsumer(c)

			f(c, jobs)
		})
	}

	setup(t, func(c *Consumer, jobs *MockJobs) {
		require.NoError(t, jobs.PushN(10))
		require.NoError(t, c.Start(0))

		require.Eventually(t, func() bool {
			c.mu.Lock()
			defer c.mu.Unlock()
			return c.processedIndex == uint64(10)
		}, 2*time.Second, 10*time.Millisecond)

		// should have no processing after all jobs are processed
		c.mu.Lock()
		defer c.mu.Unlock()
		require.Len(t, c.processings, 0)
		require.Len(t, c.processingsIndex, 0)
	})
}

func assertJobs(t *testing.T, expectedIndex []uint64, jobsToRun []*jobAtIndex) {
	actualIndex := make([]uint64, 0, len(jobsToRun))
	for _, jobAtIndex := range jobsToRun {
		require.NotNil(t, jobAtIndex.job)
		actualIndex = append(actualIndex, jobAtIndex.index)
	}
	require.Equal(t, expectedIndex, actualIndex)
}

// MockJobs implements the Jobs int64erface, and is used as the dependency for
// the Consumer for testing purpose
type MockJobs struct {
	sync.Mutex
	log      zerolog.Logger
	last     int
	jobs     map[int]module.Job
	index    map[module.JobID]int
	JobMaker *JobMaker
}

func NewMockJobs() *MockJobs {
	return &MockJobs{
		log:      unittest.Logger().With().Str("module", "jobs").Logger(),
		last:     0, // must be from 1
		jobs:     make(map[int]module.Job),
		index:    make(map[module.JobID]int),
		JobMaker: NewJobMaker(),
	}
}

func (j *MockJobs) AtIndex(index uint64) (module.Job, error) {
	j.Lock()
	defer j.Unlock()

	job, ok := j.jobs[int(index)]

	j.log.Debug().Int("index", int(index)).Bool("exists", ok).Msg("reading job at index")

	if !ok {
		return nil, storage.ErrNotFound
	}

	return job, nil
}

func (j *MockJobs) Head() (uint64, error) {
	return uint64(j.last), nil
}

func (j *MockJobs) Add(job module.Job) error {
	j.Lock()
	defer j.Unlock()

	j.log.Debug().Str("job_id", string(job.ID())).Msg("adding job")

	id := job.ID()
	_, ok := j.index[id]
	if ok {
		return storage.ErrAlreadyExists
	}

	index := j.last + 1
	j.index[id] = int(index)
	j.jobs[index] = job
	j.last++

	j.log.
		Debug().Str("job_id", string(job.ID())).
		Int("index", index).
		Msg("job added at index")

	return nil
}

func (j *MockJobs) PushOne() error {
	job := j.JobMaker.Next()
	return j.Add(job)
}

func (j *MockJobs) PushN(n int64) error {
	for i := 0; i < int(n); i++ {
		err := j.PushOne()
		if err != nil {
			return err
		}
	}
	return nil
}

// deterministically compute the JobID from index
func JobIDAtIndex(index uint64) module.JobID {
	return module.JobID(fmt.Sprintf("%v", index))
}

// JobMaker is a test helper.
// it creates new job with unique job id
type JobMaker struct {
	sync.Mutex
	index uint64
}

func NewJobMaker() *JobMaker {
	return &JobMaker{
		index: 1,
	}
}

type TestJob struct {
	index uint64
}

func (tj TestJob) ID() module.JobID {
	return JobIDAtIndex(tj.index)
}

// return next unique job
func (j *JobMaker) Next() module.Job {
	j.Lock()
	defer j.Unlock()

	job := &TestJob{
		index: j.index,
	}
	j.index++
	return job
}

type mockWorker struct {
	consumer *Consumer
}

func newMockWorker() *mockWorker {
	return &mockWorker{}
}

func (w *mockWorker) WithConsumer(c *Consumer) {
	w.consumer = c
}

func (w *mockWorker) Run(job module.Job) error {
	w.consumer.NotifyJobIsDone(job.ID())
	return nil
}
