package verification

import (
	"crypto/rand"

	"github.com/dapperlabs/flow-go/model/flow"
)

// RandomERGen generates a random ExecutionReceipt solely for unit testing purposes
// The following fields of the generated receipt are chosen randomly
// PreviousExecutionResultHash
// BlockHash
// FinalStateCommitment
// The rest of bytes are chosen as nil
func RandomERGen() *flow.ExecutionReceipt {
	var resultID flow.Identifier
	var blockID flow.Identifier
	var executorID flow.Identifier
	stateComm := make([]byte, 32)
	executorSignature := make([]byte, 32)
	// NOTE: these never error
	_, _ = rand.Read(resultID[:])
	_, _ = rand.Read(blockID[:])
	_, _ = rand.Read(executorID[:])
	_, _ = rand.Read(stateComm)
	_, _ = rand.Read(executorSignature)

	exeResult := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID:     resultID,
			BlockID:              blockID,
			FinalStateCommitment: stateComm,
			Chunks:               flow.ChunkList{},
		},
		Signatures: nil,
	}

	return &flow.ExecutionReceipt{
		ExecutorID:        executorID,
		ExecutionResult:   exeResult,
		Spocks:            nil,
		ExecutorSignature: executorSignature,
	}
}

// RandRAGen receives an ExecutionReceipt (ER) and generates a ResultApproval (RA)
// for it.
// In the current implementation, it just attaches hash of the ER to
// the RA, and leaves the other fields of the RA empty.
func RandomRAGen(receipt *flow.ExecutionReceipt) *flow.ResultApproval {
	return &flow.ResultApproval{
		ResultApprovalBody: flow.ResultApprovalBody{
			ExecutionResultID:    receipt.ExecutionResult.ID(),
			AttestationSignature: nil,
			ChunkIndexList:       nil,
			Proof:                nil,
			Spocks:               nil,
		},
		VerifierSignature: nil,
	}
}
