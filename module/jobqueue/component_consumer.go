package jobqueue

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type ComponentConsumer struct {
	component.Component

	cm           *component.ComponentManager
	consumer     module.JobConsumer
	jobs         module.Jobs
	workSignal   <-chan struct{}
	preNotifier  NotifyDone
	postNotifier NotifyDone
	log          zerolog.Logger
}

// NewComponentConsumer creates a new ComponentConsumer consumer
func NewComponentConsumer(
	log zerolog.Logger,
	workSignal <-chan struct{},
	progress storage.ConsumerProgress,
	jobs module.Jobs,
	defaultIndex uint64,
	processor JobProcessor, // method used to process jobs
	maxProcessing uint64,
	maxSearchAhead uint64,
) (*ComponentConsumer, error) {

	c := &ComponentConsumer{
		workSignal: workSignal,
		jobs:       jobs,
		log:        log,
	}

	// create a worker pool with maxProcessing workers to process jobs
	worker := NewWorkerPool(
		processor,
		func(id module.JobID) { c.NotifyJobIsDone(id) },
		maxProcessing,
	)

	consumer, err := NewConsumer(log, jobs, progress, worker, maxProcessing, maxSearchAhead, defaultIndex)
	if err != nil {
		return nil, err
	}
	c.consumer = consumer

	builder := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			c.log.Info().Msg("job consumer starting")
			err := c.consumer.Start()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not start consumer: %w", err))
			}

			ready()

			<-ctx.Done()
			c.log.Info().Msg("job consumer shutdown started")

			// blocks until all running jobs have stopped
			c.consumer.Stop()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			worker.Start(ctx)

			select {
			case <-ctx.Done():
				c.log.Info().Msg("job consumer startup aborted")
			case <-worker.Ready():
				ready()
			}

			<-worker.Done()
			c.log.Info().Msg("job consumer shutdown complete")
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// marking this ready first allows this worker to depend on the component's own Ready()
			// channel to detect when other workers have started
			ready()

			select {
			case <-ctx.Done():
				return
			case <-c.Ready():
				c.processingLoop(ctx)
			}
		})

	cm := builder.Build()
	c.cm = cm
	c.Component = cm

	return c, nil
}

// SetPreNotifier sets a notification function that is invoked before marking a job as done in the
// consumer.
//
// Note: This guarantees that the function is called at least once for each job, but may be executed
// before consumer updates the last processed index.
func (c *ComponentConsumer) SetPreNotifier(fn NotifyDone) {
	c.preNotifier = fn
}

// SetPostNotifier sets a notification function that is invoked after marking a job as done in the
// consumer.
//
// Note: This guarantees that the function is executed after consumer updates the last processed index,
// but notifications may be missed in the event of a crash.
func (c *ComponentConsumer) SetPostNotifier(fn NotifyDone) {
	c.postNotifier = fn
}

// NotifyJobIsDone is invoked by the worker to let the consumer know that it is done
// processing a (block) job.
func (c *ComponentConsumer) NotifyJobIsDone(jobID module.JobID) uint64 {
	if c.preNotifier != nil {
		c.preNotifier(jobID)
	}

	// notify wrapped consumer that job is complete
	processedIndex := c.consumer.NotifyJobIsDone(jobID)

	if c.postNotifier != nil {
		c.postNotifier(jobID)
	}

	return processedIndex
}

// Size returns number of in-memory block jobs that block consumer is processing.
func (c *ComponentConsumer) Size() uint {
	return c.consumer.Size()
}

// Head returns the highest job index available
func (c *ComponentConsumer) Head() (uint64, error) {
	return c.jobs.Head()
}

// LastProcessedIndex returns the last processed job index
func (c *ComponentConsumer) LastProcessedIndex() uint64 {
	return c.consumer.LastProcessedIndex()
}

func (c *ComponentConsumer) processingLoop(ctx irrecoverable.SignalerContext) {
	c.log.Debug().Msg("listening for new jobs")
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.workSignal:
			c.consumer.Check()
		}
	}
}
