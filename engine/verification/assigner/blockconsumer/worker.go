package blockconsumer

import (
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
)

// worker is an internal type of this package.
// It receives block jobs from job consumer and converts it to Block and passes it to the
// finalized block processor (i.e., assigner engine) to process.
// In this sense, worker acts as a broker between the block consumer and block processor.
// The worker is stateless, and is solely responsible for converting block jobs to blocks, passing them
// to the processor and notifying consumer when the block is processed.
type worker struct {
	processor assigner.FinalizedBlockProcessor
	consumer  *BlockConsumer
}

func newWorker(processor assigner.FinalizedBlockProcessor) *worker {
	return &worker{
		processor: processor,
	}
}

func (w *worker) withBlockConsumer(consumer *BlockConsumer) {
	w.consumer = consumer
}

// Run is a block worker that receives a job corresponding to a finalized block.
// It then converts the job to a block and passes it to the underlying engine
// for processing.
func (w *worker) Run(job module.Job) error {
	block, err := jobqueue.JobToBlock(job)
	if err != nil {
		return err
	}
	// TODO: wire out the internal fatal error, and return.
	w.processor.ProcessFinalizedBlock(block)
	return nil
}

// Notify is a callback for engine to notify a block has been
// processed by the given blockID.
// The worker translates the block ID into job ID and notifies the consumer
// that the job is done.
func (w *worker) Notify(blockID flow.Identifier) {
	jobID := jobqueue.JobID(blockID)
	w.consumer.NotifyJobIsDone(jobID)
}
