package execution

import "github.com/onflow/flow-go/model/flow"

// Checker checks whether the executed block's state commitment is consistent with
// the seals
type Checker interface {
	CheckExecutionConsistencyWithBlock(executed *flow.Block, finalState flow.StateCommitment)
}
