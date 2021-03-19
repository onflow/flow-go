package processor

import (
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// Worker receives job from job consumer and converts it back to Block
// for engine to process
// Worker is stateless, it acts as a middleman to convert the job into
// the entity that the engine is expecting, and translating the id of
// the entity back to JobID notify the consumer a job is done.
type Worker struct {
	blockWorker assigner.FinalizedBlockProcessor
	consumer    *BlockConsumer
}

func newWorker(blockWorker assigner.FinalizedBlockProcessor) *Worker {
	return &Worker{blockWorker: blockWorker}
}

func (w *Worker) withBlockConsumer(consumer *BlockConsumer) {
	w.consumer = consumer
}

// Run is a block worker that receives a job corresponding to a finalized block.
// It then converts the job to a block and passes it to the underlying engine
// for processing.
func (w *Worker) Run(job module.Job) {
	block := jobToBlock(job)
	w.blockWorker.ProcessFinalizedBlock(block)
}

// Notify is a callback for engine to notify a block has been
// processed by the given blockID
// the worker will translate the block ID into jobID and notify the consumer
// that the job is done.
func (w *Worker) Notify(blockID flow.Identifier) {
	jobID := blockIDToJobID(blockID)
	w.consumer.NotifyJobIsDone(jobID)
}
