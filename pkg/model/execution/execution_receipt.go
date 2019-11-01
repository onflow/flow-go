package execution

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

type Spock []byte

// this is the part is publishable
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

// CanonicalEncoding returns the encoded canonical execution receipt as bytes.
func (er *ExecutionReceipt) CanonicalEncoding() []byte {
	// TODO (Ramtin) add more fields here
	return nil
}

// Hash generates hash of the execution receipt
func (er *ExecutionReceipt) Hash() crypto.Hash {
	// TODO
	return []byte("ExecutionReceiptHash")
}
