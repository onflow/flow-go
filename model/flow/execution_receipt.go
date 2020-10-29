package flow

import (
	"github.com/onflow/flow-go/crypto"
)

type Spock []byte

// ExecutionReceiptMeta contains the metadata the distinguishes an execution
// receipt from an execution result. This is used for storing results and
// receipts separately in a composable way.
type ExecutionReceiptMeta struct {
	ExecutorID        Identifier
	ResultID          Identifier
	Spocks            []crypto.Signature
	ExecutorSignature crypto.Signature
}

func ExecutionReceiptFromMeta(meta ExecutionReceiptMeta, result ExecutionResult) *ExecutionReceipt {
	return &ExecutionReceipt{
		ExecutorID:        meta.ExecutorID,
		ExecutionResult:   result,
		Spocks:            meta.Spocks,
		ExecutorSignature: meta.ExecutorSignature,
	}
}

type ExecutionReceipt struct {
	ExecutorID        Identifier
	ExecutionResult   ExecutionResult
	Spocks            []crypto.Signature
	ExecutorSignature crypto.Signature
}

// Meta returns the receipt metadata for the receipt.
func (er *ExecutionReceipt) Meta() *ExecutionReceiptMeta {
	return &ExecutionReceiptMeta{
		ExecutorID:        er.ExecutorID,
		ResultID:          er.ExecutionResult.ID(),
		Spocks:            er.Spocks,
		ExecutorSignature: er.ExecutorSignature,
	}
}

// Body returns the body of the execution receipt.
func (er *ExecutionReceipt) Body() interface{} {
	return struct {
		ExecutorID      Identifier
		ExecutionResult ExecutionResult
		Spocks          []crypto.Signature
	}{
		ExecutorID:      er.ExecutorID,
		ExecutionResult: er.ExecutionResult,
		Spocks:          er.Spocks,
	}
}

// ID returns the canonical ID of the execution receipt.
func (er *ExecutionReceipt) ID() Identifier {
	return MakeID(er.Body())
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceipt) Checksum() Identifier {
	return MakeID(er)
}
