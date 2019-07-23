package processor

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
)

// Effects is an interface for external encapuslated funcs with side-effects to be used in the receipt processor. It follows the template pattern.
type Effects interface {
	IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error)
	HasMinStake(*types.ExecutionReceipt) (bool, error)
	IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error)
	Send(*types.ExecutionReceipt, []byte) error
	SlashExpiredReceipt(*types.ExecutionReceipt) error
	SlashInvalidReceipt(*types.ExecutionReceipt, *types.BlockPartExecutionResult) error
	HandleError(error)
}
