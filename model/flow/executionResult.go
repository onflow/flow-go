package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model"
)

type ExecutionResultBody struct {
	PreviousExecutionResult model.Commit    // commit of the previous ER
	Block                   model.Commit    // commit of the current block
	FinalStateCommitment    StateCommitment // final state commitment
	Chunks                  ChunkList
}

type ExecutionResult struct {
	ExecutionResultBody
	Signatures []crypto.Signature
}

// TODO
func (er *ExecutionResult) Commit() model.Commit {
	return nil
}
