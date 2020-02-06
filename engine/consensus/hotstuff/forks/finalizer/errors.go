package finalizer

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// ErrorBlockHashCollision [FATAL] represents the case where two blocks have the same hash,
// but different properties. Processing logic has the option to raise a ErrorBlockHashCollision
// if it comes across a pair of blocks with same ID (Hash) but different fields.
//
// Theoretically, there are two potential reasons for this error:
// (1) the block is invalid (manipulated) but has passed the validity check (bug);
// (2) there is a true hash collision. This is extre..e-17..emly unlikely
type ErrorBlockHashCollision struct {
	block1   *types.BlockProposal
	block2   *types.BlockProposal
	location string
}

func (e *ErrorBlockHashCollision) Error() string {
	return fmt.Sprintf(
		"Got two blocks different views %d and %d but same ID %v",
		e.block1.View(), e.block2.View(), e.block1.BlockID(),
	)
}

// ErrorPruned3Chain sentinel error
type ErrorPrunedAncestry struct {
	block *types.BlockProposal
}

func (e *ErrorPrunedAncestry) Error() string {
	return fmt.Sprintf(
		"Block with view %d and ID %s has pruned 3-chain history",
		e.block.View(), e.block.BlockID(),
	)
}
