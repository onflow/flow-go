// Package compute holds validation result models.
// It is currently a temporary place holder.
// Once the validation flow is hooked up with the actual compute module, this package will likely be removed or changed.
package compute

import "github.com/dapperlabs/bamboo-node/pkg/types"

type BlockPartExecutionResult struct {
	PartIndex        uint64
	PartTransactions []types.IntermediateRegisters
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
