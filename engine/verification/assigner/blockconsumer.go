package assigner

import (
	"github.com/onflow/flow-go/model/flow"
)

// FinalizedBlockWorker should be implemented by the verification node engine responsible
// for processing execution receipts included in a finalized block.
type FinalizedBlockWorker interface {
	ProcessFinalizedBlock(*flow.Block)
}
