package execution

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/storage"
)

type ExecutionResult struct {
	PreviousExecutionResultHash crypto.Hash
	BlockHash                   crypto.Hash
	FinalStateCommitment        storage.StateCommitment
	Chunks                      []Chunk
	Signatures                  []crypto.Signature
}

// Hash returns the canonical hash of this execution result.
func (er *ExecutionResult) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(er.Encode())
}

// Encode returns the canonical encoding of this execution result.
func (er *ExecutionResult) Encode() []byte {
	w := wrapExecutionResult(*er)
	return encoding.DefaultEncoder.MustEncode(&w)
}

type ExecutionResultWrapper struct {
	PreviousExecutionResultHash crypto.Hash
	BlockHash                   crypto.Hash
	FinalStateCommitment        storage.StateCommitment
	Chunks                      []chunkWrapper
}

func wrapExecutionResult(er ExecutionResult) ExecutionResultWrapper {
	chunks := make([]chunkWrapper, len(er.Chunks))
	for i, ck := range er.Chunks {
		chunks[i] = wrapChunk(ck)
	}
	return ExecutionResultWrapper{
		PreviousExecutionResultHash: er.PreviousExecutionResultHash,
		BlockHash:                   er.BlockHash,
		FinalStateCommitment:        er.FinalStateCommitment,
		Chunks:                      chunks}
}
