package jobqueue

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Consumer struct {
	sync.Mutex
	log zerolog.Logger

	// storage
	jobs     storage.Jobs
	progress storage.ConsumerProgress

	// config
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

func NewConsumer(log zerolog.Logger, jobs storage.Jobs, progress storage.ConsumerProgress, maxProcessing int, maxPending int, fn func(job Job)) *Consumer {
	return &Consumer{
		log: log,

		// store dependency
		jobs:     jobs,
		progress: progress,

		// update config
		maxProcessing: maxProcessing,
		maxPending:    maxPending,
		fn:            fn,

		// init state variables
		running:          false,
		processedIndex:   0,
		processings:      make(map[int]*JobStatus),
		processingsIndex: make(map[JobID]int),
	}
}

type JobStatus struct {
	jobID JobID
	done  bool
}

func (c *Consumer) Start() error {
	c.Lock()
	defer c.Unlock()

	if c.running {
		return nil
	}

	c.running = true

	// on startup, sync with storage for the processed index
	// to ensure the consistency
	processedIndex, err := c.progress.ProcessedIndex()
	if err != nil {
		return fmt.Errorf("could not read processed index: %w", err)
	}
	c.processedIndex = processedIndex

	c.checkProcessable()

	return nil
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
	c.Lock()
	defer c.Unlock()

	c.checkProcessable()
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

	processables, processedIndex, err := c.processableJobs()
	if err != nil {
		return 0, fmt.Errorf("could not query processable jobs: %w", err)
	}

	for _, job := range processables {
		go c.fn(job)
	}

	err = c.progress.SetProcessedIndex(processedIndex)
	if err != nil {
		return 0, fmt.Errorf("could not set processed index %v, %w", processedIndex, err)
	}
	c.processedIndex = processedIndex

	return len(processables), nil
}

// processableJobs check the worker's capacity and if sufficient, read
// jobs from the storage, return the processable jobs, and the processed
// index
func (c *Consumer) processableJobs() ([]Job, int, error) {
	processables := make([]Job, 0)

	// count how many jobs are still processing,
	// in order to decide whether to process a new job
	processing := 0
	pending := 0
	processedIndex := c.processedIndex

	// if still have processing capacity, find the next processable
	// job
	for i := processedIndex + 1; processing <= c.maxProcessing && pending <= c.maxPending; i++ {
		status, ok := c.processings[i]

		// if no one is processing the next job, try to read one
		// job and process it.
		if !ok {
			// take one job
			job, err := c.jobs.AtIndex(i)

			// if there is no more job at this index, we could stop
			if errors.Is(err, storage.ErrNotFound) {
				break
			}

			// exception
			if err != nil {
				return nil, 0, fmt.Errorf("could not read job at index %v, %w", i, err)
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

		if i == processedIndex+1 {
			processedIndex++
		} else {
			pending++
		}
	}

	return processables, processedIndex, nil
}

// saveJob only update the interanal state to add the processing job
func (c *Consumer) saveJob(jobID JobID, index int) {
	c.processingsIndex[jobID] = index
	c.processings[index] = &JobStatus{
		jobID: jobID,
		done:  false,
	}
}

// doneJob updates the internal state to mark the job has been processed
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
