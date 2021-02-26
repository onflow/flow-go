package assigner

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
func (j BlockJob) ID() module.JobID {
	return blockIDToJobID(j.Block.ID())
}

func blockIDToJobID(blockID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", blockID))
}

func blockToJob(block *flow.Block) *BlockJob {
	return &BlockJob{Block: block}
}

func jobToBlock(job module.Job) *flow.Block {
	blockJob, _ := job.(*BlockJob)
	return blockJob.Block
}

// FinalizedBlockReader provides an abstraction for consumers to read blocks
// as job
type FinalizedBlockReader struct {
	state protocol.State
}

// the job index would just be the finalized block height
func (r *FinalizedBlockReader) AtIndex(index int64) (module.Job, error) {
	var finalBlock *flow.Block
	return blockToJob(finalBlock), fmt.Errorf("to be implement")
}

// Head returns the last finalized height as job index
func (r *FinalizedBlockReader) Head() (int64, error) {
	return 0, fmt.Errorf("return the last finalized height")
}

// Worker receives job from job consumer and converts it back to Block
// for engine to process
// Worker is stateless, it acts as a middleman to convert the job into
// the entity that the engine is expecting, and translating the id of
// the entity back to JobID notify the consumer a job is done.
type Worker struct {
	engine   *Engine
	consumer *BlockConsumer
}

// BlockWorker receives a job corresponding to a finalized block.
// It then converts the job to a block and passes it to the underlying engine
// for processing.
func (w *Worker) Run(job module.Job) {
	block := jobToBlock(job)
	w.engine.ProcessFinalizedBlock(block)
}

// FinishProcessing is a callback for engine to notify a block has been
// processed by the given blockID
// the worker will translate the block ID into jobID and notify the consumer
// that the job is done.
func (w *Worker) Notify(blockID flow.Identifier) {
	jobID := blockIDToJobID(blockID)
	w.consumer.NotifyJobIsDone(jobID)
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

// NewBlockConsumer creates a new consumer and returns the default processed
// index for initializing the processed index in storage.
func NewBlockConsumer(
	log zerolog.Logger,
	processedHeight storage.ConsumerProgress,
	state protocol.State,
	engine *Engine,
	maxProcessing int64,
) (*BlockConsumer, int64, error) {
	worker := &Worker{engine: engine}
	engine.withBlockConsumerNotifier(worker)

	jobs := &FinalizedBlockReader{state: state}

	consumer := jobqueue.NewConsumer(log, jobs, processedHeight, worker, maxProcessing)

	defaultIndex, err := defaultProcessedIndex(state)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read default processed index: %w", err)
	}

	blockConsumer := &BlockConsumer{
		consumer:     consumer,
		defaultIndex: defaultIndex,
	}

	worker.consumer = blockConsumer

	return blockConsumer, defaultIndex, nil
}

func (c *BlockConsumer) NotifyJobIsDone(jobID module.JobID) {
	c.consumer.NotifyJobIsDone(jobID)
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
