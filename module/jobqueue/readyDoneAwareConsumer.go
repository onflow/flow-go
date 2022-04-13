package jobqueue

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type ReadyDoneAwareConsumer struct {
	component.Component
	module.Resumable

	cm           *component.ComponentManager
	consumer     module.JobConsumer
	jobs         module.Jobs
	workSignal   <-chan struct{}
	defaultIndex uint64
	notifier     NotifyDone
	log          zerolog.Logger
}

// NewReadyDoneAwareConsumer creates a new ReadyDoneAwareConsumer consumer
func NewReadyDoneAwareConsumer(
	log zerolog.Logger,
	progress storage.ConsumerProgress,
	jobs module.Jobs,
	processor JobProcessor, // method used to process jobs
	workSignal <-chan struct{},
	defaultIndex uint64,
	maxProcessing uint64,
	notifier NotifyDone,
) (*ReadyDoneAwareConsumer, error) {

	c := &ReadyDoneAwareConsumer{
		defaultIndex: defaultIndex,
		workSignal:   workSignal,
		notifier:     notifier,
		jobs:         jobs,
		log:          log,
	}

	worker := NewReadyDoneAwareWorker(
		processor,
		func(id module.JobID) { c.NotifyJobIsDone(id) },
		maxProcessing,
	)
	c.consumer = NewConsumer(c.log, c.jobs, progress, worker, maxProcessing)

	builder := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			worker.Start(ctx)
			<-worker.Ready()
			c.log.Info().Msg("job consumer starting")

			err := c.consumer.Start(c.defaultIndex)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not start consumer: %w", err))
			}

			ready()

			<-ctx.Done()
			c.log.Info().Msg("job consumer shutdown started")

			// blocks until all running jobs have stopped
			c.consumer.Stop()

			<-worker.Done()
			c.log.Info().Msg("job consumer shutdown complete")
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			c.processingLoop(ctx)
		})

	cm := builder.Build()
	c.cm = cm
	c.Component = cm
	c.Resumable = c.consumer

	return c, nil
}

// NotifyJobIsDone is invoked by the worker to let the consumer know that it is done
// processing a (block) job.
func (c *ReadyDoneAwareConsumer) NotifyJobIsDone(jobID module.JobID) uint64 {
	// notify wrapped consumer that job is complete
	c.defaultIndex = c.consumer.NotifyJobIsDone(jobID)

	// notify instantiator that job is complete
	if c.notifier != nil {
		c.notifier(jobID)
	}

	return c.defaultIndex
}

// Size returns number of in-memory block jobs that block consumer is processing.
func (c *ReadyDoneAwareConsumer) Size() uint {
	return c.consumer.Size()
}

// Head returns the highest job index available
func (c *ReadyDoneAwareConsumer) Head() (uint64, error) {
	return c.jobs.Head()
}

func (c *ReadyDoneAwareConsumer) processingLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.workSignal:
			c.consumer.Check()
		}
	}
}
