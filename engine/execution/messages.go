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
