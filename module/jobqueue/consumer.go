package jobqueue

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/storage"
)

type Worker interface {
	Run(job storage.Job)
}

type Consumer struct {
	sync.Mutex
	log zerolog.Logger

	// Storage
	jobs     storage.Jobs             // storage to read jobs from
	progress storage.ConsumerProgress // storing the last processed job, so that we can resume after restarting
	worker   Worker                   // defines how jobs will be processed

	// Config
	maxProcessing int64 // max number of jobs to be processed concurrently

	// State Variables
	running bool // a signal to control whether to start processing more jobs. Useful for waiting
	// until the workers are ready
	isChecking *atomic.Bool // allow only one process checking job processable
	// are ready, and stop when shutting down.

	processedIndex   int64
	processings      map[int64]*jobStatus // keep track of the status of each on going job
	processingsIndex map[JobID]int64      // lookup the index of the job, useful when fast forwarding the
	// `processed` variable
}

func NewConsumer(
	log zerolog.Logger,
	jobs storage.Jobs,
	progress storage.ConsumerProgress,
	worker Worker,
	maxProcessing int64,
) *Consumer {
	return &Consumer{
		log: log,

		// store dependency
		jobs:     jobs,
		progress: progress,
		worker:   worker,

		// update config
		maxProcessing: maxProcessing,

		// init state variables
		running:          false,
		isChecking:       atomic.NewBool(false),
		processedIndex:   0,
		processings:      make(map[int64]*jobStatus),
		processingsIndex: make(map[JobID]int64),
	}
}

// Start starts consuming the jobs from the job queue.
func (c *Consumer) Start(defaultIndex int64) error {
	c.Lock()
	defer c.Unlock()

	if c.running {
		return nil
	}

	c.running = true

	// on startup, sync with storage for the processed index
	// to ensure the consistency
	processedIndex, err := c.progress.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		ok, err := c.progress.InitProcessedIndex(defaultIndex)
		if err != nil {
			return fmt.Errorf("could not init processed index: %w", err)
		}

		if !ok {
			return fmt.Errorf("init processed index had no effect, default index: %v",
				defaultIndex)
		}

		processedIndex = defaultIndex

		c.log.Warn().Int64("processed index", processedIndex).
			Msg("processed index not found, initialized.")
	} else if err != nil {
		return fmt.Errorf("could not read processed index: %w", err)
	}

	c.processedIndex = processedIndex

	c.checkProcessable()

	c.log.Info().Int64("processed", processedIndex).Msg("consumer started")
	return nil
}

// Stop stops consuming jobs from the job queue.
// Note, it won't stop the existing worker from finishing their job
func (c *Consumer) Stop() {
	c.Lock()
	defer c.Unlock()

	c.running = false
	c.log.Info().Msg("consumer stopped")
}

// FinishJob let the consumer know a job has been finished, so that consumer will take
// the next job from the job queue if there are workers available
func (c *Consumer) FinishJob(jobID JobID) {
	c.Lock()
	defer c.Unlock()
	c.log.Debug().Str("job_id", string(jobID)).Msg("finishing job")

	if c.doneJob(jobID) {
		c.checkProcessable()
	}
}

// Check allows the job publisher to notify the consumer that a new job has been added, so that
// the consumer can check if the job is processable
// since multiple checks at the same time are unnecessary, we could only keep one check by checking.
// an atomic isChecking value.
func (c *Consumer) Check() {
	if !c.isChecking.CAS(false, true) {
		// other process is checking, we could exit and rely on that process to check
		// processable jobs
		return
	}

	// still need to lock here, since checkProcessable might update the state vars.
	c.Lock()
	defer c.Unlock()

	c.checkProcessable()

	c.isChecking.Store(false)
}

// checkProcessable is a wrap of the `run` function with logging
func (c *Consumer) checkProcessable() {
	c.log.Debug().Msg("checking processable jobs")

	processingCount, err := c.run()
	if err != nil {
		c.log.Error().Err(err).Msg("failed to check processables")
		return
	}

	if processingCount > 0 {
		c.log.Info().Int64("processing", processingCount).Msg("processing jobs")
	} else {
		c.log.Debug().Bool("running", c.running).Msg("no job found")
	}

}

// run checks if there are processable jobs and process them by giving
// them to the callback functions.
// this function is passive, it won't trigger itself, but can only be
// triggered by either Start or FinishJob
func (c *Consumer) run() (int64, error) {
	if !c.running {
		return 0, nil
	}

	processedFrom := c.processedIndex
	processables, processedTo, err := c.processableJobs()
	if err != nil {
		return 0, fmt.Errorf("could not query processable jobs: %w", err)
	}

	c.log.Debug().
		Int64("processed_from", processedFrom).
		Int64("processed_to", processedTo).
		Int("processables", len(processables)).
		Msg("running")

	for _, jobAtIndex := range processables {
		jobID := jobAtIndex.job.ID()

		c.processingsIndex[jobID] = jobAtIndex.index
		c.processings[jobAtIndex.index] = &jobStatus{
			jobID: jobID,
			done:  false,
		}

		go c.worker.Run(jobAtIndex.job)
	}

	err = c.progress.SetProcessedIndex(processedTo)
	if err != nil {
		return 0, fmt.Errorf("could not set processed index %v, %w", processedTo, err)
	}
	c.processedIndex = processedTo

	return int64(len(processables)), nil
}

func (c *Consumer) processableJobs() ([]*jobAtIndex, int64, error) {
	return processableJobs(
		c.jobs,
		c.processings,
		c.maxProcessing,
		c.processedIndex,
	)
}

// processableJobs check the worker's capacity and if sufficient, read
// jobs from the storage, return the processable jobs, and the processed
// index
func processableJobs(jobs storage.Jobs, processings map[int64]*jobStatus, maxProcessing int64, processedIndex int64) ([]*jobAtIndex, int64, error) {
	processables := make([]*jobAtIndex, 0)

	// count how many jobs are still processing,
	// in order to decide whether to process a new job
	processing := int64(0)

	// if still have processing capacity, find the next processable job
	for i := processedIndex + 1; processing < maxProcessing; i++ {
		status, ok := processings[i]

		// if no worker is processing the next job, try to read it and process
		if !ok {
			// take one job
			job, err := jobs.AtIndex(i)

			// if there is no more job at this index, we could stop
			if errors.Is(err, storage.ErrNotFound) {
				break
			}

			// exception
			if err != nil {
				return nil, 0, fmt.Errorf("could not read job at index %v, %w", i, err)
			}

			processing++

			processables = append(processables, &jobAtIndex{
				job:   job,
				index: i,
			})
			continue
		}

		// only increment the processing variable when
		// the job is not done, meaning still processing
		if !status.done {
			processing++
			continue
		}

		if i == processedIndex+1 {
			processedIndex++
		}
	}

	return processables, processedIndex, nil
}

// doneJob updates the internal state to mark the job has been processed
// return true if the job is changed from processing to finished
// return false if the job is already finished, or removed
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
		c.log.Fatal().Msgf("bug, job (%v) can not be found by index (%v)", jobID, index)
		return false
	}

	if status.done {
		// job has been done already
		return false
	}

	status.done = true
	return true
}

type jobAtIndex struct {
	job   Job
	index int64
}

type jobStatus struct {
	jobID JobID
	done  bool
}
