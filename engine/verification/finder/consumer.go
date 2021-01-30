package finder

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type BlockJob struct {
	Block *flow.Block
}

func (j *BlockJob) ID() module.JobID {
	return module.JobID(fmt.Sprintf("%v", j.Block.ID()))
}

type FinalizedBlockReader struct {
	state protocol.State
}

func (r *FinalizedBlockReader) AtIndex(index int64) (storage.Job, error) {
	return nil, fmt.Errorf("to implement")
}

type Worker struct {
	engine *Engine
}

// BlockWorker takes the job which contains a block header, and work on it.
// it checks whether it needs to be processed.
// it should not work on a sealed block, it should not work on a block that is not staked.
// then, it reads all the receipts from the block payload. For each receipt, it checks if
// there is any chunk assigned to me, and store all these chunks in another chunk job queue
func (w *Worker) Run(job storage.Job) {
	blockjob, _ := job.(*BlockJob)
	w.engine.ProcessFinalizedBlock(blockjob.Block)
}

// BlockConsumer listens to the OnFinalizedBlock event
// and notify the consumer to Check in the job queue
type BlockConsumer struct {
	consumer module.JobConsumer
}

func NewBlockConsumer(log zerolog.Logger, processedHeight storage.ConsumerProgress, state protocol.State, engine *Engine, maxProcessing int64, maxFinished int64) *BlockConsumer {
	worker := &Worker{engine: engine}
	jobs := &FinalizedBlockReader{state: state}

	consumer := jobqueue.NewConsumer(
		log, jobs, processedHeight, worker, maxProcessing, maxFinished,
	)

	return &BlockConsumer{
		consumer: consumer,
	}
}

func (c *BlockConsumer) OnFinalizedBlock(block *model.Block) {
	c.consumer.Check()
}

func (c *BlockConsumer) Ready() <-chan struct{} {
	c.consumer.Start()

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
