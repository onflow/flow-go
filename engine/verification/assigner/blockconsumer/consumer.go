package blockconsumer

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DefaultBlockWorkers is the number of blocks processed in parallel.
const DefaultBlockWorkers = uint64(2)

// BlockConsumer listens to the OnFinalizedBlock event
// and notifies the consumer to check in the job queue
// (i.e., its block reader) for new block jobs.
type BlockConsumer struct {
	consumer module.JobConsumer
	unit     *engine.Unit
	metrics  module.VerificationMetrics
}

// NewBlockConsumer creates a new consumer
func NewBlockConsumer(log zerolog.Logger,
	metrics module.VerificationMetrics,
	processedHeight storage.ConsumerProgress,
	blocks storage.Blocks,
	state protocol.State,
	blockProcessor assigner.FinalizedBlockProcessor,
	maxProcessing uint64,
	finalizationRegistrar hotstuff.FinalizationRegistrar,
) (*BlockConsumer, error) {

	lg := log.With().Str("module", "block_consumer").Logger()

	// wires blockProcessor as the worker. The block consumer will
	// invoke instances of worker concurrently to process block jobs.
	worker := newWorker(blockProcessor)
	blockProcessor.WithBlockConsumerNotifier(worker)

	// the block reader is where the consumer reads new finalized blocks from (i.e., jobs).
	jobs := jobqueue.NewFinalizedBlockReader(state, blocks)

	consumer, err := jobqueue.NewConsumer(lg, jobs, processedHeight, worker, maxProcessing, 0)
	if err != nil {
		return nil, fmt.Errorf("could not create block consumer: %w", err)
	}

	blockConsumer := &BlockConsumer{
		consumer: consumer,
		unit:     engine.NewUnit(),
		metrics:  metrics,
	}
	worker.withBlockConsumer(blockConsumer)

	// register callback with finalization registrar
	finalizationRegistrar.AddOnBlockFinalizedConsumer(blockConsumer.onFinalizedBlock)

	return blockConsumer, nil
}

// NotifyJobIsDone is invoked by the worker to let the consumer know that it is done
// processing a (block) job.
func (c *BlockConsumer) NotifyJobIsDone(jobID module.JobID) {
	processedIndex := c.consumer.NotifyJobIsDone(jobID)
	c.metrics.OnBlockConsumerJobDone(processedIndex)
}

// Size returns number of in-memory block jobs that block consumer is processing.
func (c *BlockConsumer) Size() uint {
	return c.consumer.Size()
}

// onFinalizedBlock implements FinalizationConsumer, and is invoked by the follower engine whenever
// a new block is finalized.
// In this implementation for block consumer, invoking OnFinalizedBlock is enough to only notify the consumer
// to check its internal queue and move its processing index ahead to the next height if there are workers available.
// The consumer retrieves the new blocks from its block reader module, hence it does not need to use the parameter
// of OnFinalizedBlock here.
func (c *BlockConsumer) onFinalizedBlock(*model.Block) {
	c.unit.Launch(c.consumer.Check)
}

func (c *BlockConsumer) Ready() <-chan struct{} {
	err := c.consumer.Start()
	if err != nil {
		panic(fmt.Errorf("could not start block consumer for finder engine: %w", err))
	}

	ready := make(chan struct{})
	close(ready)
	return ready
}

func (c *BlockConsumer) Done() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		completeChan := c.unit.Done()
		c.consumer.Stop()
		<-completeChan
		close(ready)
	}()
	return ready
}
