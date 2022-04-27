package requester

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

type BlockExecutionDataConsumer func(blockHeight uint64, executionData *execution_data.BlockExecutionData)

type Requester struct {
	notifier   *notifier
	dispatcher *dispatcher
}

// AddConsumer adds a new block execution data consumer.
// The returned function can be used to remove the consumer.
// Note: the addition and removal of consumers happens asynchronously, so the
// consumer may not begin receiving callbacks immediately after this function
// returns, and may not stop receiving callbacks immediately after the returned
// removal function is called.
func (r *Requester) AddConsumer(consumer BlockExecutionDataConsumer) func() {
	// TODO: check for shutdown
	return r.notifier.subscribe(&notificationSub{consumer})
}

func (r *Requester) HandleReceipt(receipt *flow.ExecutionReceipt) {
	// TODO

}

func (r *Requester) HandleFinalizedBlock(block *model.Block) {
	// TODO
}

// TODO: we first start the notifier, then the fulfiller, then the handler, then the dispatcher.
// Or, we just let them wait for each other to be ready.

// Then when we start the Requester, we use RunCompoent?
