package types

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type ExecutionReceipt struct {
	PreviousReceiptHash       crypto.Hash
	BlockHash                 crypto.Hash
	InitialRegisters          []Register
	IntermediateRegistersList []IntermediateRegisters
	Signatures                []crypto.Signature
}

// Encode matches crypto.Encoder interface
func (m *ExecutionReceipt) Encode() []byte {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(m); err != nil {
		panic(err)
	}
	return b.Bytes()
}

type InvalidExecutionReceiptChallenge struct {
	ExecutionReceiptHash      crypto.Hash
	ExecutionReceiptSignature crypto.Signature
	PartIndex                 uint64
	PartTransactions          []IntermediateRegisters
	Signature                 crypto.Signature
}

// TODO: this type should be defined inside the compute library.
// It is given here in the meanwhile.
type BlockPartExecutionResult struct {
	PartIndex        uint64
	PartTransactions []IntermediateRegisters
}

type ResultApproval struct {
	BlockHeight             uint64
	ExecutionReceiptHash    crypto.Hash
	ResultApprovalSignature crypto.Signature
	Proof                   uint64
	Signature               crypto.Signature
}
