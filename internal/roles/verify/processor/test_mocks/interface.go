package testmocks

import (
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/processor"
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
	IsValidExecutionReceipt(*processor.ExecutionReceipt) (compute.ValidationResult, error)
	HasMinStake(*processor.ExecutionReceipt) (bool, error)
	IsSealedWithDifferentReceipt(*processor.ExecutionReceipt) (bool, error)
	Send(*processor.ExecutionReceipt, []byte) error
	SlashExpiredReceipt(*processor.ExecutionReceipt) error
	SlashInvalidReceipt(*processor.ExecutionReceipt, *compute.BlockPartExecutionResult) error
	HandleError(error)
}
