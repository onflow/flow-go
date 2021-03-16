package assigner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// FinalizedBlockProcessor should be implemented by the verification node engine responsible
// for processing execution receipts included in a finalized block.
type FinalizedBlockProcessor interface {
	ProcessFinalizedBlock(*flow.Block)

	// WithBlockConsumerNotifier sets the notifier of the block processor.
	// The notifier is called by the internal logic of the processor to let the consumer know that
	// the processor is done by processing a block so that the next block may be passed to the processor
	// by the consumer through invoking ProcessFinalizedBlock of this processor.
	WithBlockConsumerNotifier(module.ProcessingNotifier)
}
