package execution

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

type ComputationResult struct {
	*BlockExecutionResults
	*BlockAttestationResults

	*flow.ExecutionReceipt
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	ber := NewPopulatedBlockExecutionResults(block)
	aer := NewEmptyBlockAttestationResults(ber)
	return &ComputationResult{
		BlockExecutionResults:   ber,
		BlockAttestationResults: aer,
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
