package assigner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// FinalizedBlockProcessor should be implemented by the verification node engine responsible
// for processing execution receipts included in a finalized block.
//
// In the current version, the assigner engine is responsible of processing execution receipts
// included in a finalized block.
// From the architectural perspective, FinalizedBlockProcessor aligns as following on the verification pipeline:
// -----------------------------                           ------------------                                  --------------------------
// | Consensus Follower Engine | ---> finalized blocks --> | Block Consumer | ---> finalized block workers --> | FinalizedBlockProcessor|
// -----------------------------                           ------------------                                  --------------------------
type FinalizedBlockProcessor interface {
	// ProcessFinalizedBlock receives a finalized block and processes all of its constituent execution receipts.
	// Note: it should be implemented in a non-blocking way.
	ProcessFinalizedBlock(*flow.Block)

	// WithBlockConsumerNotifier sets the notifier of this finalized block processor.
	// The notifier is called by the internal logic of the processor to let the consumer know that
	// the processor is done by processing a block so that the next block may be passed to the processor
	// by the consumer through invoking ProcessFinalizedBlock of this processor.
	WithBlockConsumerNotifier(module.ProcessingNotifier)
}
