package compute

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
)

type ValidationResult interface {
	isValidationResult()
}

type ValidationResultSuccess struct {
	Proof []byte
	ValidationResult
}

type ValidationResultFail struct {
	BlockPartResult *types.BlockPartExecutionResult
	ValidationResult
}
