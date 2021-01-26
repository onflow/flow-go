package jobqueue

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProcessableJobs(t *testing.T) {
	maxProcessing := 3
	maxFinished := 8

	t.Run("no job, nothing to process", func(t *testing.T) {
		jobs := NewMockJobs() // no job in the queue
		processings := map[int]*JobStatus{}
		processedIndex := 0

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 0, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("neither max processing or max finished was reached", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 11; i++ {
			// job 3 are 5 are not done, 2 processing in total
			// 4, 6, 7, 8, 9, 10, 11 are finished, 7 finished in total
			done := true
			if i == 3 || i == 5 {
				done = false
			}
			processings[i] = &JobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		// it will process on more job, and reach the max processing.
		assertJobs(t, []int{
			12,
		}, jobsToRun)
	})

	t.Run("only reached max processing", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 12; i++ {
			// job 3, 5, 6 are not done, which have reached max processing(3)
			// 4, 7, 8, 9, 10, 11, 12 are finished, 7 finished in total, haven't reached max finished (8)
			done := true
			if i == 3 || i == 5 || i == 6 {
				done = false
			}
			processings[i] = &JobStatus{jobID: JobIDAtIndex(i), done: done}
		}
		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		// it will not process any job, because the max processing is reached.
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("only reached max finished", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 12; i++ {
			// job 3 and 4 are not done, which have not reached max processing (3)
			// 5, 6, 7, 8, 9, 10, 11, 12 are finished, 8 finished in total, which have reached max finished
			done := true
			if i == 3 || i == 4 {
				done = false
			}
			processings[i] = &JobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("reached both max processing and max finished", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 13; i++ {
			// job 3, 4, 13 are not done, which have reached max processing (3)
			// 5, 6, 7, 8, 9, 10, 11, 12 are finished, 8 finished in total, which have reached max finished
			done := true
			if i == 3 || i == 4 || i == 13 {
				done = false
			}
			processings[i] = &JobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("no more job", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(11)) // 11 jobs, no more job to process
		processings := map[int]*JobStatus{}
		for i := 3; i <= 11; i++ {
			// job 3, 11 are not done, which have not reached max processing (3)
			// 4, 5, 6, 7, 8, 9, 10 are finished, 7 finished in total, which have reached max finished
			done := true
			if i == 3 || i == 11 {
				done = false
			}
			processings[i] = &JobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("next jobs were done", func(t *testing.T) {
		jobs := NewMockJobs()
		require.NoError(t, jobs.PushN(20)) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 6; i++ {
			// job 3, 5 are done, which have not reached max finished
			// job 4, 6 are not done, which have not reached max processing
			done := true
			if i == 4 || i == 6 {
				done = false
			}
			processings[i] = &JobStatus{jobID: JobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxFinished, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 3, processedTo)
		assertJobs(t, []int{
			7,
		}, jobsToRun)
	})
}

func assertJobs(t *testing.T, expectedIndex []int, jobsToRun []*jobAtIndex) {
	actualIndex := make([]int, 0, len(jobsToRun))
	for _, jobAtIndex := range jobsToRun {
		require.NotNil(t, jobAtIndex.job)
		actualIndex = append(actualIndex, jobAtIndex.index)
	}
	require.Equal(t, expectedIndex, actualIndex)
}

// MockJobs implements the Jobs interface, and is used as the dependency for
// the Consumer for testing purpose
type MockJobs struct {
	sync.Mutex
	log      zerolog.Logger
	last     int
	jobs     map[int]Job
	index    map[JobID]int
	JobMaker *JobMaker
}

func NewMockJobs() *MockJobs {
	return &MockJobs{
		log:      unittest.Logger().With().Str("module", "jobs").Logger(),
		last:     0, // must be from 1
		jobs:     make(map[int]Job),
		index:    make(map[JobID]int),
		JobMaker: NewJobMaker(),
	}
}

// var _ storage.Jobs = &MockJobs{}

func (j *MockJobs) AtIndex(index int) (storage.Job, error) {
	j.Lock()
	defer j.Unlock()

	job, ok := j.jobs[index]

	j.log.Debug().Int("index", index).Bool("exists", ok).Msg("reading job at index")

	if !ok {
		return nil, storage.ErrNotFound
	}

	return job, nil
}

func (j *MockJobs) Add(job storage.Job) error {
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

func (j *MockJobs) PushOne() error {
	job := j.JobMaker.Next()
	return j.Add(job)
}

func (j *MockJobs) PushN(n int) error {
	for i := 0; i < n; i++ {
		err := j.PushOne()
		if err != nil {
			return err
		}
	}
	return nil
}

// deterministically compute the JobID from index
func JobIDAtIndex(index int) storage.JobID {
	return storage.JobID(fmt.Sprintf("%v", index))
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

func (tj TestJob) ID() storage.JobID {
	return JobIDAtIndex(tj.index)
}

// return next unique job
func (j *JobMaker) Next() storage.Job {
	j.Lock()
	defer j.Unlock()

	job := &TestJob{
		index: j.index,
	}
	j.index++
	return job
}
