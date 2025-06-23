package execution

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

type ComputationResult struct {
	*BlockExecutionResult
	*BlockAttestationResult

	*flow.ExecutionReceipt
}

// NewEmptyComputationResult creates an empty ComputationResult.
// Construction ComputationResult allowed only within the constructor.
func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
	versionAwareChunkConstructor flow.ChunkConstructor,
) *ComputationResult {
	ber := NewPopulatedBlockExecutionResult(block)
	aer := NewEmptyBlockAttestationResult(ber, versionAwareChunkConstructor)
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
