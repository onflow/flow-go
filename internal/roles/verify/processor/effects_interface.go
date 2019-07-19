package processor

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
)

// processorEffects is an interface for external encapuslated funcs with side-effects to be used in the receipt processor. Follows the GoF template pattern.
type processorEffects interface {
	isValidExecutionReceipt(*types.ExecutionReceipt) (ValidationResult, error)
	hasMinStake(*types.ExecutionReceipt) (bool, error)
	isSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error)
	send(*types.ExecutionReceipt, []byte) error
	slashExpiredReceipt(*types.ExecutionReceipt) error
	slashInvalidReceipt(*types.ExecutionReceipt, *types.BlockPartExecutionResult) error
	handleError(error)
}
