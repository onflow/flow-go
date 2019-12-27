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
	previousER := make([]byte, 32)
	blockHash := make([]byte, 32)
	stateComm := make([]byte, 32)
	executorSignature := make([]byte, 32)
	rand.Read(previousER)
	rand.Read(blockHash)
	rand.Read(stateComm)
	rand.Read(executorSignature)

	exeResult := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousExecutionResult: blockHash,
			Block:                   stateComm,
			FinalStateCommitment:    nil,
			Chunks:                  flow.ChunkList{},
		},
		Signatures: nil,
	}

	return &flow.ExecutionReceipt{
		ExecutionResult:   exeResult,
		Spocks:            nil,
		ExecutorSignature: executorSignature,
	}
}

// RandRAGen receives an ExecutionReceipt (ER) and generates a ResultApproval (RA)
// for it.
// In the current implementation, it just attaches hash of the ER to
// the RA, and leaves the other fields of the RA empty.
func RnadRAGen(receipt *flow.ExecutionReceipt) *flow.ResultApproval {
	return 	&flow.ResultApproval{
		Body:              flow.ResultApprovalBody{
			ExecutionResultHash: receipt.ExecutionResult.Fingerprint(),
			AttestationSignature: nil,
			ChunkIndexList:       nil,
			Proof:                nil,
			Spocks:               nil,
		},
		VerifierSignature: nil,
	}
}
