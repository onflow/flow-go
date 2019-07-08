package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type ExecutionReceipt struct {
	PreviousReceiptHash       crypto.Hash
	BlockHash                 crypto.Hash
	InitialRegisters          []Register
	IntermediateRegistersList []IntermediateRegisters
	Signatures                []crypto.Signature
}

type InvalidExecutionReceiptChallenge struct {
	ExecutionReceiptHash      crypto.Hash
	ExecutionReceiptSignature crypto.Signature
	PartIndex                 int64
	PartTransactions          []IntermediateRegisters
	Signature                 crypto.Signature
}

type ResultApproval struct {
	BlockHeight             uint64
	ExecutionReceiptHash    crypto.Hash
	ResultApprovalSignature crypto.Signature
	Proof                   uint64
	Signature               crypto.Signature
}
