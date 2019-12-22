package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
)

type ExecutionResultBody struct {
	PreviousExecutionResult Fingerprint     // commit of the previous ER
	Block                   Fingerprint     // commit of the current block
	FinalStateCommitment    StateCommitment // final state commitment
	Chunks                  ChunkList
}

type ExecutionResult struct {
	ExecutionResultBody
	Signatures []crypto.Signature
}

func (er *ExecutionResult) Fingerprint() Fingerprint {
	return encoding.DefaultEncoder.MustEncode(er.ExecutionResultBody)
}

