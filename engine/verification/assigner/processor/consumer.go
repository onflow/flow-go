package processor

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
// and notify the consumer to Check in the job queue
type BlockConsumer struct {
	consumer     module.JobConsumer
	defaultIndex int64
}

// the consumer is consuming the jobs from jobs queue for the first time,
// we initialize the default processed index as the last sealed block's height,
// so that we will process from the first unsealed block
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

	worker := newWorker(blockProcessor)
	blockProcessor.WithBlockConsumerNotifier(worker)
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

func (c *BlockConsumer) NotifyJobIsDone(jobID module.JobID) {
	c.consumer.NotifyJobIsDone(jobID)
}

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
