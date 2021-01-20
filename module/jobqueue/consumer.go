package jobqueue

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/storage"
)

type Consumer struct {
	sync.Mutex
	log     zerolog.Logger
	storage *badger.DB

	// config
	jobName       []byte
	maxProcessing int
	maxPending    int
	fn            func(job Job)

	// state variables
	running        bool
	processedIndex int
	// the processing maintains the status
	// it also useful when fast forwarding the `processed`
	// variable, as we need to lookup jobs by index
	processings      map[int]*JobStatus
	processingsIndex map[JobID]int
}

func NewConsumer(log zerolog.Logger, storage *badger.DB, maxProcessing int, maxPending int, jobName []byte, fn func(job Job), processedIndex int) *Consumer {
	return &Consumer{
		storage:          storage,
		log:              log,
		jobName:          jobName,
		maxProcessing:    maxProcessing,
		maxPending:       maxPending,
		fn:               fn,
		running:          false,
		processedIndex:   processedIndex,
		processings:      make(map[int]*JobStatus),
		processingsIndex: make(map[JobID]int),
	}
}

type JobStatus struct {
	jobID JobID
	done  bool
}

func (c *Consumer) Start() {
	c.Lock()
	defer c.Unlock()

	if c.running {
		return
	}

	c.running = true

	c.checkProcessable()
}

func (c *Consumer) Stop() {
	c.Lock()
	defer c.Unlock()

	c.running = false
}

func (c *Consumer) FinishJob(jobID JobID) {
	c.Lock()
	defer c.Unlock()

	if c.doneJob(jobID) {
		c.checkProcessable()
	}
}

func (c *Consumer) Check() {
}

// checkProcessable is a wrap of the `run` function with logging
func (c *Consumer) checkProcessable() {
	processingCount, err := c.run()
	if err != nil {
		log.Error().Err(err).Msg("failed to check processables")
		return
	}

	if processingCount > 0 {
		log.Debug().Int("processing", processingCount).Msg("job started")
	}
}

// run checks if there are processable jobs and process them by giving
// them to the callback functions.
// this function is passive, it won't trigger itself, but can only be
// triggered by either Start or FinishJob
func (c *Consumer) run() (int, error) {
	if !c.running {
		return 0, nil
	}

	processables, err := c.processableJobs()
	if err != nil {
		return 0, fmt.Errorf("could not query processable jobs: %w", err)
	}

	for _, job := range processables {
		go c.fn(job)
	}

	return len(processables), nil
}

func (c *Consumer) processableJobs() ([]Job, error) {
	processables := make([]Job, 0)

	// count how many jobs are still processing,
	// in order to decide whether to process a new job
	processing := 0
	pending := 0

	// if still have processing capacity, find the next processable
	// job
	for i := c.processedIndex + 1; processing <= c.maxProcessing && pending <= c.maxPending; i++ {
		status, ok := c.processings[i]

		// if no one is processing the next job, try to read one
		// job and process it.
		if !ok {
			// take one job
			job, err := c.readJobAtIndex(i)

			// if there is no more job at this index, we could stop
			if errors.Is(err, storage.ErrNotFound) {
				break
			}

			// exception
			if err != nil {
				return nil, fmt.Errorf("could not read job at index %v, %w", i, err)
			}

			jobID := job.ID()

			c.saveJob(jobID, i)
			processing++

			processables = append(processables, job)
		}

		// only increment the processing variable when
		// the job is not done, meaning still processing
		if !status.done {
			processing++
			continue
		}

		if i == c.processedIndex+1 {
			err := c.setProcessedJob(i)
			if err != nil {
				return nil, fmt.Errorf("could not write processed index %v, %w", i, err)
			}
			c.processedIndex = i
		} else {
			pending++
		}
	}

	return processables, nil
}

func (c *Consumer) saveJob(jobID JobID, index int) {
	c.processingsIndex[jobID] = index
	c.processings[index] = &JobStatus{
		jobID: jobID,
		done:  false,
	}
}

func (c *Consumer) doneJob(jobID JobID) bool {
	// lock
	index, ok := c.processingsIndex[jobID]
	if !ok {
		// job must has been processed
		return false
	}

	status, ok := c.processings[index]
	if !ok {
		// must be a bug, if went here
		return false
	}

	if status.done {
		// job has been done already
		return false
	}

	status.done = true
	return true
}

func (c *Consumer) readJobAtIndex(index int) (Job, error) {
	// key := c.jobName + index
	// bytes, err := c.storage.Get(key)
	var job Job
	var err error
	if err != nil {
		return nil, fmt.Errorf("could not get job: %w", err)
	}

	return job, nil
}

func (c *Consumer) setProcessedJob(index int) error {
	return nil
}
