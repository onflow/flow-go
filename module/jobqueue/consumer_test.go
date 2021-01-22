package jobqueue

import (
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestProcessableJobs(t *testing.T) {
	maxProcessing := 3
	maxPending := 8

	t.Run("no job, nothing to process", func(t *testing.T) {
		jobs := newMockJobs() // no job in the queue
		processings := map[int]*JobStatus{}
		processedIndex := 0

		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 0, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("neither max processing or max pending was reached", func(t *testing.T) {
		jobs := newMockJobs()
		jobs.PushN(20) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 11; i++ {
			// job 3 are 5 are not done, 2 processing in total
			// 4, 6, 7, 8, 9, 10, 11 are pending, 7 pending in total
			done := true
			if i == 3 || i == 5 {
				done = false
			}
			processings[i] = &JobStatus{jobID: jobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		// it will process on more job, and reach the max processing.
		assertJobs(t, []int{
			12,
		}, jobsToRun)
	})

	t.Run("only reached max processing", func(t *testing.T) {
		jobs := newMockJobs()
		jobs.PushN(20) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 12; i++ {
			// job 3, 5, 6 are not done, which have reached max processing(3)
			// 4, 7, 8, 9, 10, 11, 12 are pending, 7 pending in total, haven't reached max pending (8)
			done := true
			if i == 3 || i == 5 || i == 6 {
				done = false
			}
			processings[i] = &JobStatus{jobID: jobIDAtIndex(i), done: done}
		}
		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		// it will not process any job, because the max processing is reached.
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("only reached max pending", func(t *testing.T) {
		jobs := newMockJobs()
		jobs.PushN(20) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 12; i++ {
			// job 3 and 4 are not done, which have not reached max processing (3)
			// 5, 6, 7, 8, 9, 10, 11, 12 are pending, 8 pending in total, which have reached max pending
			done := true
			if i == 3 || i == 4 {
				done = false
			}
			processings[i] = &JobStatus{jobID: jobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("reached both max processing and max pending", func(t *testing.T) {
		jobs := newMockJobs()
		jobs.PushN(20) // enough jobs in the queue
		processings := map[int]*JobStatus{}
		for i := 3; i <= 13; i++ {
			// job 3, 4, 13 are not done, which have reached max processing (3)
			// 5, 6, 7, 8, 9, 10, 11, 12 are pending, 8 pending in total, which have reached max pending
			done := true
			if i == 3 || i == 4 || i == 13 {
				done = false
			}
			processings[i] = &JobStatus{jobID: jobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("no more job", func(t *testing.T) {
		jobs := newMockJobs()
		jobs.PushN(11) // 11 jobs, no more job to process
		processings := map[int]*JobStatus{}
		for i := 3; i <= 11; i++ {
			// job 3, 11 are not done, which have not reached max processing (3)
			// 4, 5, 6, 7, 8, 9, 10 are pending, 7 pending in total, which have reached max pending
			done := true
			if i == 3 || i == 11 {
				done = false
			}
			processings[i] = &JobStatus{jobID: jobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

		require.NoError(t, err)
		require.Equal(t, 2, processedTo)
		assertJobs(t, []int{}, jobsToRun)
	})

	t.Run("next jobs were done", func(t *testing.T) {
		jobs := newMockJobs()
		jobs.PushN(20) // enough jobs
		processings := map[int]*JobStatus{}
		for i := 3; i <= 6; i++ {
			// job 3, 5 are done, which have not reached max pending
			// job 4, 6 are not done, which have not reached max processing
			done := true
			if i == 4 || i == 6 {
				done = false
			}
			processings[i] = &JobStatus{jobID: jobIDAtIndex(i), done: done}
		}

		processedIndex := 2
		jobsToRun, processedTo, err := processableJobs(jobs, processings, maxProcessing, maxPending, processedIndex)

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

func (j *mockJobs) AtIndex(index int) (storage.Job, error) {
	j.Lock()
	defer j.Unlock()

	job, ok := j.jobs[index]

	j.log.Debug().Int("index", index).Bool("exists", ok).Msg("reading job at index")

	if !ok {
		return nil, storage.ErrNotFound
	}

	return job, nil
}

func (j *mockJobs) Add(job storage.Job) error {
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

type testJob struct {
	index int
}

func (tj testJob) ID() storage.JobID {
	return jobIDAtIndex(tj.index)
}

// return next unique job
func (j *jobMaker) Next() storage.Job {
	j.Lock()
	defer j.Unlock()

	job := &testJob{
		index: j.index,
	}
	j.index++
	return job
}
