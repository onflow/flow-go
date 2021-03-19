package jobqueue

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProcessableJobs(t *testing.T) {
	t.Parallel()

	maxProcessing := int64(3)

	t.Run("no job, nothing to process", func(t *testing.T) {
		jobs := NewMockJobs() // no job in the queue
		processings := map[int64]*jobStatus{}
		processedIndex := int64(0)

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, int64(0), processedTo)
		assertJobs(t, []int64{}, jobsToRun)
	})

	t.Run("max processing was not reached", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int64]*jobStatus{}
		for i := 3; i <= 11; i++ {
			// job 3 are 5 are not done, 2 processing in total
			// 4, 6, 7, 8, 9, 10, 11 are finished, 7 finished in total
			done := true
			if i == 3 || i == 5 {
				done = false
			}
			processings[int64(i)] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := int64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, int64(2), processedTo)
		// it will process on more job, and reach the max processing.
		assertJobs(t, []int64{
			12,
		}, jobsToRun)
	})

	t.Run("reached max processing", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int64]*jobStatus{}
		for i := 3; i <= 12; i++ {
			// job 3, 5, 6 are not done, which have reached max processing(3)
			// 4, 7, 8, 9, 10, 11, 12 are finished, 7 finished in total
			done := true
			if i == 3 || i == 5 || i == 6 {
				done = false
			}
			processings[int64(i)] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}
		processedIndex := int64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, int64(2), processedTo)
		// it will not process any job, because the max processing is reached.
		assertJobs(t, []int64{}, jobsToRun)
	})

	t.Run("no more job", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(11)) // 11 jobs, no more job to process
		processings := map[int64]*jobStatus{}
		for i := 3; i <= 11; i++ {
			// job 3, 11 are not done, which have not reached max processing (3)
			// 4, 5, 6, 7, 8, 9, 10 are finished, 7 finished in total
			done := true
			if i == 3 || i == 11 {
				done = false
			}
			processings[int64(i)] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := int64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, int64(2), processedTo)
		assertJobs(t, []int64{}, jobsToRun)
	})

	t.Run("next jobs were done", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int64]*jobStatus{}
		for i := 3; i <= 6; i++ {
			// job 3, 5 are done
			// job 4, 6 are not done, which have not reached max processing
			done := true
			if i == 4 || i == 6 {
				done = false
			}
			processings[int64(i)] = &jobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := int64(2)
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, processedIndex)

		require.NoError(t, err)
		require.Equal(t, int64(3), processedTo)
		assertJobs(t, []int64{
			7,
		}, jobsToRun)
	})
}

func assertJobs(t *testing.T, expectedIndex []int64, jobsToRun []*jobAtIndex) {
	actualIndex := make([]int64, 0, len(jobsToRun))
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

func (j *MockJobs) AtIndex(index int64) (module.Job, error) {
	j.Lock()
	defer j.Unlock()

	job, ok := j.jobs[int(index)]

	j.log.Debug().Int("index", int(index)).Bool("exists", ok).Msg("reading job at index")

	if !ok {
		return nil, storage.ErrNotFound
	}

	return job, nil
}

func (j *MockJobs) Head() (int64, error) {
	return int64(j.last), nil
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
func JobIDAtIndex(index int) module.JobID {
	return module.JobID(fmt.Sprintf("%v", index))
}

// JobMaker is a test helper.
// it creates new job with unique job id
type JobMaker struct {
	sync.Mutex
	index int
}

func NewJobMaker() *JobMaker {
	return &JobMaker{
		index: 1,
	}
}

type TestJob struct {
	index int
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
