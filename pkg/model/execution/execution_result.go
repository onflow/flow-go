package execution

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type ExecutionResult struct {
	PreviousExecutionResultHash crypto.Hash
	BlockHash                   crypto.Hash
	FinalStateCommitment        StateCommitment
	Chunks                      []Chunk
	Signatures                  []crypto.Signature
}

// Encode implements the crypto.Encoder interface.
func (er *ExecutionResult) Encode() []byte {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(er); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// CanonicalEncoding returns the encoded canonical execution receipt as bytes.
func (er *ExecutionResult) CanonicalEncoding() []byte {
	var items []interface{}
	items = append(items, er.PreviousExecutionResultHash)
	items = append(items, er.BlockHash)
	items = append(items, er.FinalStateCommitment)
	for _, chunk := range er.Chunks {
		items = append(items, chunk.Hash())
	}
	b, _ := rlp.EncodeToBytes(items)
	return b
}

// Hash computes the hash over the necessary transaction data.
func (er *ExecutionResult) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	return hasher.ComputeHash(er.CanonicalEncoding())
}
