package testmocks

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
)

// Mock interface provides a set of common methods to introspect the mock instance
type Mock interface {
	MockCounters
	ProcessorEffects
}

type MockCounters interface {
	CallCountIsValidExecutionReceipt() int
	CallCountHasMinStake() int
	CallCountIsSealedWithDifferentReceipt() int
	CallCountSend() int
	CallCountSlashExpiredReceipt() int
	CallCountSlashInvalidReceipt() int
	CallCountHandleError() int
}

// ProcessorEffects matches processor.Effects interface.
// The latter cannot be reused here due to circular import.
type ProcessorEffects interface {
	IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error)
	HasMinStake(*types.ExecutionReceipt) (bool, error)
	IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error)
	Send(*types.ExecutionReceipt, []byte) error
	SlashExpiredReceipt(*types.ExecutionReceipt) error
	SlashInvalidReceipt(*types.ExecutionReceipt, *types.BlockPartExecutionResult) error
	HandleError(error)
}
