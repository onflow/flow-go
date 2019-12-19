package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model"
)

type ExecutionResultBody struct {
	PreviousExecutionResult model.Fingerprint // commit of the previous ER
	Block                   model.Fingerprint // commit of the current block
	FinalStateCommitment    StateCommitment   // final state commitment
	Chunks                  ChunkList
}

type ExecutionResult struct {
	ExecutionResultBody
	Signatures []crypto.Signature
}

// TODO
func (er *ExecutionResult) Fingerprint() model.Fingerprint {
	return nil
}
