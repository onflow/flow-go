package execution

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

type ExecutionResult struct {
	PreviousExecutionResultHash crypto.Hash
	BlockHash                   crypto.Hash
	FinalStateCommitment        StateCommitment
	Chunks                      []Chunk
	Signatures                  []crypto.Signature
}

// Hash returns the canonical hash of this execution result.
func (er *ExecutionResult) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(er.Encode())
}

// Encode returns the canonical encoding of this execution result.
func (er *ExecutionResult) Encode() []byte {
	w := WrapExecutionResult(*er)
	return encoding.DefaultEncoder.MustEncode(&w)
}

type ExecutionResultWrapper struct {
	PreviousExecutionResultHash crypto.Hash
	BlockHash                   crypto.Hash
	FinalStateCommitment        StateCommitment
	Chunks                      []chunkWrapper
}

func WrapExecutionResult(er ExecutionResult) ExecutionResultWrapper {
	chunks := make([]chunkWrapper, len(er.Chunks))
	for i, ck := range er.Chunks {
		chunks[i] = WrapChunk(ck)
	}
	return ExecutionResultWrapper{
		PreviousExecutionResultHash: er.PreviousExecutionResultHash,
		BlockHash:                   er.BlockHash,
		FinalStateCommitment:        er.FinalStateCommitment,
		Chunks:                      chunks}
}
