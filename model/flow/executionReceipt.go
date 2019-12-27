package flow

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/hash"
)

type Spock []byte

type ExecutionReceipt struct {
	ExecutionResult   ExecutionResult
	Spocks            []Spock
	ExecutorSignature crypto.Signature
}

// Encode implements the crypto.Encoder interface.
func (er *ExecutionReceipt) Encode() []byte {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(er); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// Hash returns the canonical hash of this execution receipt.
func (er *ExecutionReceipt) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(er.Encode())
}
