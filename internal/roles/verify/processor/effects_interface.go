package processor

import (
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
)

// Effects is an interface for external encapuslated funcs with side-effects to be used in the receipt processor. It follows the template pattern.
type Effects interface {
	IsValidExecutionReceipt(*ExecutionReceipt) (compute.ValidationResult, error)
	HasMinStake(*ExecutionReceipt) (bool, error)
	IsSealedWithDifferentReceipt(*ExecutionReceipt) (bool, error)
	Send(*ExecutionReceipt, []byte) error
	SlashExpiredReceipt(*ExecutionReceipt) error
	SlashInvalidReceipt(*ExecutionReceipt, *compute.BlockPartExecutionResult) error
	HandleError(error)
}
