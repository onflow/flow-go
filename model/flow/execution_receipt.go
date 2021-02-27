package flow

import (
	"github.com/onflow/flow-go/crypto"
)

type Spock []byte

// ExecutionReceipt is the full execution receipt, as sent by the Execution Node.
// Specifically, it contains the detailed execution result.
type ExecutionReceipt struct {
	ExecutorID        Identifier
	ExecutionResult   ExecutionResult
	Spocks            []crypto.Signature
	ExecutorSignature crypto.Signature
}

// ID returns the canonical ID of the execution receipt.
func (er *ExecutionReceipt) ID() Identifier {
	return er.Meta().ID()
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceipt) Checksum() Identifier {
	return MakeID(er)
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

// ExecutionReceiptMeta contains the fields from the Execution Receipts
// that vary from one executor to another (assuming they commit to the same
// result). It only contains the ID (cryptographic hash) of the execution
// result the receipt commits to. The ExecutionReceiptMeta is useful for
// storing results and receipts separately in a composable way.
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

// ID returns the canonical ID of the execution receipt.
// It is identical to the ID of the full receipt.
func (er *ExecutionReceiptMeta) ID() Identifier {
	body := struct {
		ExecutorID Identifier
		ResultID   Identifier
		Spocks     []crypto.Signature
	}{
		ExecutorID: er.ExecutorID,
		ResultID:   er.ResultID,
		Spocks:     er.Spocks,
	}
	return MakeID(body)
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceiptMeta) Checksum() Identifier {
	return MakeID(er)
}
