// Package compute holds validation result models.
// It is currently a temporary place holder.
// Once the validation flow is hooked up with the actual compute module, this package will likely be removed or changed.
package compute

import (
	"crypto"
)

type Registers map[crypto.Hash][]byte

type IntermediateRegisters struct {
	TransactionHash crypto.Hash
	Registers       Registers
	ComputeUsed     uint64
}

// TODO: this type should be defined inside the compute library.
// It is given here in the meanwhile.
type BlockPartExecutionResult struct {
	PartIndex        uint64
	PartTransactions []IntermediateRegisters
}

type ValidationResult interface {
	isValidationResult()
}

type ValidationResultSuccess struct {
	Proof []byte
	ValidationResult
}

type ValidationResultFail struct {
	BlockPartResult *BlockPartExecutionResult
	ValidationResult
}
