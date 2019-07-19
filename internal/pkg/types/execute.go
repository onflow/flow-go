package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/utils/slices"
)

type ExecutionReceipt struct {
	PreviousReceiptHash       crypto.Hash
	BlockHash                 crypto.Hash
	InitialRegisters          []Register
	IntermediateRegistersList []IntermediateRegisters
	Signatures                []crypto.Signature
}

// Encode is defined to match crypto.Encoder interface
func (m *ExecutionReceipt) Encode() []byte {
	var b [][]byte
	// todo: fix once crypto package is merged
	b = [][]byte{
		m.PreviousReceiptHash[:],
	}
	return slices.Concat(b)

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
