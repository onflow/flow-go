package jobqueue

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProcessableJobs(t *testing.T) {
	t.Parallel()

	processedIndex := uint64(2)
	maxProcessing := uint64(3)
	maxSearchAhead := uint64(5)

	populate := func(start, end uint64, incomplete []uint64) map[uint64]*jobStatus {
		processings := map[uint64]*jobStatus{}
		for i := start; i <= end; i++ {
			processings[i] = &jobStatus{jobID: JobIDAtIndex(i), done: true}
		}
		for _, i := range incomplete {
			processings[i].done = false
		}

		return processings
	}

	t.Run("no job, nothing to process", func(t *testing.T) {
		jobs := NewMockJobs() // no job in the queue
		processings := map[uint64]*jobStatus{}
		processedIndex := uint64(0)

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, 0, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(0), processedTo)
		assertJobs(t, []uint64{}, jobsToRun)
	})

	t.Run("max processing was not reached", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue

		// job 3 are 5 are not done, 2 processing in total
		// 4, 6, 7, 8, 9, 10, 11 are finished, 7 finished in total
		processings := populate(3, 11, []uint64{3, 5})

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, 0, processedIndex)

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

		// job 3, 5, 6 are not done, which have reached max processing(3)
		// 4, 7, 8, 9, 10, 11, 12 are finished, 7 finished in total
		processings := populate(3, 12, []uint64{3, 5, 6})

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, 0, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(2), processedTo)
		// it will not process any job, because the max processing is reached.
		assertJobs(t, []uint64{}, jobsToRun)
	})

	t.Run("processing pauses and resumes", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue

		maxProcessing := uint64(4)

		// job 3, 5 are not done
		// 4, 6, 7 are finished, 3 finished in total
		processings := populate(3, processedIndex+maxSearchAhead, []uint64{3, 5})

		// it will not process any job, because the consumer is paused
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxSearchAhead, processedIndex)

		require.NoError(t, err)
		require.Equal(t, processedIndex, processedTo)
		assertJobs(t, []uint64{}, jobsToRun)

		// lowest job is processed, which should cause consumer to resume
		processings[uint64(3)].done = true

		// Job 3 is done, so it should return 2 more jobs 8-9 and pause again with one available worker
		jobsToRun, processedTo, err = processableJobs(jobs, processings, maxProcessing, maxSearchAhead, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(4), processedTo)
		assertJobs(t, []uint64{8, 9}, jobsToRun)

		// lowest job is processed, which should cause consumer to resume
		processings[uint64(5)].done = true

		// job 5 is processed, it should return jobs 8-11 (one job for each worker)
		jobsToRun, processedTo, err = processableJobs(jobs, processings, maxProcessing, maxSearchAhead, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(7), processedTo)
		assertJobs(t, []uint64{8, 9, 10, 11}, jobsToRun)
	})

	t.Run("no more job", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(11)) // 11 jobs, no more job to process

		// job 3, 11 are not done, which have not reached max processing (3)
		// 4, 5, 6, 7, 8, 9, 10 are finished, 7 finished in total
		processings := populate(3, 11, []uint64{3, 11})

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, 0, processedIndex)

		require.NoError(t, err)
		require.Equal(t, uint64(2), processedTo)
		assertJobs(t, []uint64{}, jobsToRun)
	})

	t.Run("next jobs were done", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue

		// job 3, 5 are done
		// job 4, 6 are not done, which have not reached max processing
		processings := populate(3, 6, []uint64{4, 6})

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, 0, processedIndex)

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
			c, err := NewConsumer(log, jobs, progress, worker, maxProcessing, 0, 0)
			require.NoError(t, err)
			worker.WithConsumer(c)

			f(c, jobs)
		})
	}

	setup(t, func(c *Consumer, jobs *MockJobs) {
		require.NoError(t, jobs.PushN(10))
		require.NoError(t, c.Start())

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

func TestCheckBeforeStartIsNoop(t *testing.T) {
	t.Parallel()

	unittest.RunWithBadgerDB(t, func(db *badgerdb.DB) {
		storedProcessedIndex := uint64(100)

		worker := newMockWorker()
		progress := badger.NewConsumerProgress(db, "consumer")
		err := progress.InitProcessedIndex(storedProcessedIndex)
		require.NoError(t, err)

		c, err := NewConsumer(
			unittest.Logger(),
			NewMockJobs(),
			progress,
			worker,
			uint64(3),
			0,
			10,
		)
		require.NoError(t, err)
		worker.WithConsumer(c)

		// check will store the processedIndex. Before start, it will be uninitialized (0)
		c.Check()

		// start will load the processedIndex from storage
		err = c.Start()
		require.NoError(t, err)

		// make sure that the processedIndex at the end is from storage
		assert.Equal(t, storedProcessedIndex, c.LastProcessedIndex())
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

func JobIDToIndex(id module.JobID) (uint64, error) {
	return strconv.ParseUint(string(id), 10, 64)
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
