package finder

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// BlockJob converts a Block into a Job to be used by job queue
type BlockJob struct {
	Block *flow.Block
}

// ID converts block id into job id, which guarantees uniqueness
func (j *BlockJob) ID() module.JobID {
	return blockIDToJobID(j.Block.ID())
}

func blockIDToJobID(blockID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", blockID))
}

func blockToJob(block *flow.Block) *BlockJob {
	return &BlockJob{Block: block}
}

func jobToBlock(job storage.Job) *flow.Block {
	blockJob, _ := job.(*BlockJob)
	return blockJob.Block
}

// FinalizedBlockReader provide an abstraction for consumers to read blocks
// as job
type FinalizedBlockReader struct {
	state protocol.State
}

// the job index would just be the finalized block height
func (r *FinalizedBlockReader) AtIndex(index int64) (storage.Job, error) {
	var finalBlock *flow.Block
	return blockToJob(finalBlock), fmt.Errorf("to be implement")
}

// Worker receives job from job consumer and converts it back to Block
// for engine to process
type Worker struct {
	engine   *Engine
	consumer *BlockConsumer
}

// BlockWorker takes the job which contains a block header, and work on it.
// it checks whether it needs to be processed.
// it should not work on a sealed block, it should not work on a block that is not staked.
// then, it reads all the receipts from the block payload. For each receipt, it checks if
// there is any chunk assigned to me, and store all these chunks in another chunk job queue
func (w *Worker) Run(job storage.Job) {
	block := jobToBlock(job)
	w.engine.ProcessFinalizedBlock(block)
}

func (w *Worker) FinishProcessing(blockID flow.Identifier) {
	jobID := blockIDToJobID(blockID)
	w.consumer.FinishJob(jobID)
}

// finishProcessing is for the worker's underneath engine to report a chunk
// has been processed without knowing the job queue
// it's a callback so that the worker can convert the chunk id into a job
// id, and notify the consumer about a finished job with the
type finishProcessing interface {
	FinishProcessing(chunkID flow.Identifier)
}

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

func NewBlockConsumer(
	log zerolog.Logger,
	processedHeight storage.ConsumerProgress,
	state protocol.State,
	engine *Engine,
	maxProcessing int64,
	maxFinished int64,
) (*BlockConsumer, error) {
	worker := &Worker{engine: engine}
	engine.withFinishProcessing(worker)

	jobs := &FinalizedBlockReader{state: state}

	consumer := jobqueue.NewConsumer(
		log, jobs, processedHeight, worker, maxProcessing, maxFinished,
	)

	defaultIndex, err := defaultProcessedIndex(state)
	if err != nil {
		return nil, fmt.Errorf("could not read default processed index: %w", err)
	}

	blockConsumer := &BlockConsumer{
		consumer:     consumer,
		defaultIndex: defaultIndex,
	}

	worker.consumer = blockConsumer

	return blockConsumer, nil
}

func (c *BlockConsumer) FinishJob(jobID module.JobID) {
	c.consumer.FinishJob(jobID)
}

func (c *BlockConsumer) OnFinalizedBlock(block *model.Block) {
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
