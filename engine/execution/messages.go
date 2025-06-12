package execution

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// ComputationResult captures artifacts of execution of block collections, collection attestation results and
// the full execution receipt, as sent by the Execution Node.
//
//structwrite:immutable - mutations allowed only within the constructor
type ComputationResult struct {
	*BlockExecutionResult
	*BlockAttestationResult

	*flow.ExecutionReceipt
}

// UntrustedComputationResult is an untrusted input-only representation of a ComputationResult,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedComputationResult should be validated and converted into
// a trusted ComputationResult using NewComputationResult constructor.
type UntrustedComputationResult ComputationResult

// NewComputationResult creates a new instance of ComputationResult.
// Construction ComputationResult allowed only within the constructor.
//
// All errors indicate a valid ComputationResult cannot be constructed from the input.
func NewComputationResult(untrusted UntrustedComputationResult) (*ComputationResult, error) {
	if untrusted.BlockExecutionResult == nil {
		return nil, fmt.Errorf("block execution result must not be nil")
	}
	if untrusted.BlockAttestationResult == nil {
		return nil, fmt.Errorf("block attestation result must not be nil")
	}
	return &ComputationResult{
		BlockExecutionResult:   untrusted.BlockExecutionResult,
		BlockAttestationResult: untrusted.BlockAttestationResult,
		ExecutionReceipt:       untrusted.ExecutionReceipt,
	}, nil
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
