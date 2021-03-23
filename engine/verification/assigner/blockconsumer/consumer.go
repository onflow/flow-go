package blockconsumer

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// BlockConsumer listens to the OnFinalizedBlock event
// and notifies the consumer to check in the job queue
// (i.e., its block reader) for new block jobs.
type BlockConsumer struct {
	consumer     module.JobConsumer
	defaultIndex int64
}

// defaultProcessedIndex returns the last sealed block height from the protocol state.
//
// The BlockConsumer utilizes this return height to fetch and consume block jobs from
// jobs queue the first time it initializes.
func defaultProcessedIndex(state protocol.State) (int64, error) {
	final, err := state.Sealed().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get finalized height: %w", err)
	}
	return int64(final.Height), nil
}

// NewBlockConsumer creates a new consumer and returns the default processed
// index for initializing the processed index in storage.
func NewBlockConsumer(log zerolog.Logger,
	processedHeight storage.ConsumerProgress,
	blocks storage.Blocks,
	state protocol.State,
	blockProcessor assigner.FinalizedBlockProcessor,
	maxProcessing int64) (*BlockConsumer, int64, error) {

	// wires blockProcessor as the worker. The block consumer will
	// invoke instances of worker concurrently to process block jobs.
	worker := newWorker(blockProcessor)
	blockProcessor.WithBlockConsumerNotifier(worker)

	// the block reader is where the consumer reads new finalized blocks from (i.e., jobs).
	jobs := newFinalizedBlockReader(state, blocks)

	consumer := jobqueue.NewConsumer(log, jobs, processedHeight, worker, maxProcessing)
	defaultIndex, err := defaultProcessedIndex(state)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read default processed index: %w", err)
	}

	blockConsumer := &BlockConsumer{
		consumer:     consumer,
		defaultIndex: defaultIndex,
	}
	worker.withBlockConsumer(blockConsumer)

	return blockConsumer, defaultIndex, nil
}

// NotifyJobIsDone is invoked by the worker to let the consumer know that it is done
// processing a (block) job.
func (c *BlockConsumer) NotifyJobIsDone(jobID module.JobID) {
	c.consumer.NotifyJobIsDone(jobID)
}

// OnFinalizedBlock implements FinalizationConsumer, and is invoked by the follower engine whenever
// a new block is finalized.
// In this implementation for block consumer, invoking OnFinalizedBlock is enough to only notify the consumer
// to check its internal queue and move its processing index ahead to the next height if there are workers available.
// The consumer retrieves the new blocks from its block reader module, hence it does not need to use the parameter
// of OnFinalizedBlock here.
func (c *BlockConsumer) OnFinalizedBlock(*model.Block) {
	c.consumer.Check()
}

// To implement FinalizationConsumer
func (c *BlockConsumer) OnBlockIncorporated(*model.Block) {}

// To implement FinalizationConsumer
func (c *BlockConsumer) OnDoubleProposeDetected(*model.Block, *model.Block) {}

func (c *BlockConsumer) Ready() <-chan struct{} {
	err := c.consumer.Start(c.defaultIndex)
	if err != nil {
		panic(fmt.Errorf("could not start block consumer for finder engine: %w", err))
	}

	ready := make(chan struct{})
	close(ready)
	return ready
}

func (c *BlockConsumer) Done() <-chan struct{} {
	c.consumer.Stop()

	ready := make(chan struct{})
	close(ready)
	return ready
}
