package processor

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
)

type ValidationResult interface {
	isValidationResult()
}

type ValidationResultSuccess struct {
	proof []byte
	ValidationResult
}

type ValidationResultFail struct {
	blockPartResult *types.BlockPartExecutionResult
	ValidationResult
}
