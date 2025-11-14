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

// defaultProcessedIndex returns the last sealed block height from the protocol state.
//
// The BlockConsumer utilizes this return height to fetch and consume block jobs from
// jobs queue the first time it initializes.
func defaultProcessedIndex(state protocol.State) (uint64, error) {
	final, err := state.Sealed().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get finalized height: %w", err)
	}
	return final.Height, nil
}

// NewBlockConsumer creates a new consumer and returns the default processed
// index for initializing the processed index in storage.
func NewBlockConsumer(log zerolog.Logger,
	metrics module.VerificationMetrics,
	processedHeight storage.ConsumerProgressInitializer,
	blocks storage.Blocks,
	state protocol.State,
	blockProcessor assigner.FinalizedBlockProcessor,
	maxProcessing uint64,
	followerDistributor hotstuff.Distributor) (*BlockConsumer, uint64, error) {

	lg := log.With().Str("module", "block_consumer").Logger()

	// wires blockProcessor as the worker. The block consumer will
	// invoke instances of worker concurrently to process block jobs.
	worker := newWorker(blockProcessor)
	blockProcessor.WithBlockConsumerNotifier(worker)

	// the block reader is where the consumer reads new finalized blocks from (i.e., jobs).
	jobs := jobqueue.NewFinalizedBlockReader(state, blocks)

	defaultIndex, err := defaultProcessedIndex(state)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read default processed index: %w", err)
	}

	consumer, err := jobqueue.NewConsumer(lg, jobs, processedHeight, worker, maxProcessing, 0, defaultIndex)
	if err != nil {
		return nil, 0, fmt.Errorf("could not create block consumer: %w", err)
	}

	blockConsumer := &BlockConsumer{
		consumer: consumer,
		unit:     engine.NewUnit(),
		metrics:  metrics,
	}
	worker.withBlockConsumer(blockConsumer)

	// register callback with distributor
	followerDistributor.AddOnBlockFinalizedConsumer(func(*model.Block) {
		blockConsumer.unit.Launch(blockConsumer.consumer.Check)
	})

	return blockConsumer, defaultIndex, nil
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

// OnFinalizedBlock is a no-op since callbacks are registered with the distributor in the constructor.
// This method is kept for backward compatibility.
func (c *BlockConsumer) OnFinalizedBlock(*model.Block) {
	// no-op: callback is registered with distributor in constructor
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
