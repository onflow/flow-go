package execution

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// ComputationResult captures artifacts of execution of block collections, collection attestation results and
// the full execution receipt, as sent by the Execution Node.
// CAUTION: This type is used to represent both a complete ComputationResult and a partially constructed ComputationResult.
// TODO: Consider using a Builder type to represent the partially constructed model.
type ComputationResult struct {
	*BlockExecutionResult
	*BlockAttestationResult

	ExecutionReceipt *flow.ExecutionReceipt
}

// NewEmptyComputationResult creates an empty ComputationResult.
// Construction ComputationResult allowed only within the constructor.
func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	ber := NewPopulatedBlockExecutionResult(block)
	aer := NewEmptyBlockAttestationResult(ber)
	return &ComputationResult{
		BlockExecutionResult:   ber,
		BlockAttestationResult: aer,
	}
}

// CurrentEndState returns the most recent end state
// if no attestation appended yet, it returns start state of block
// TODO(ramtin): we probably don't need this long term as part of this method
func (cr *ComputationResult) CurrentEndState() flow.StateCommitment {
	if len(cr.collectionAttestationResults) == 0 {
		return *cr.StartState
	}
	return cr.collectionAttestationResults[len(cr.collectionAttestationResults)-1].endStateCommit
}
