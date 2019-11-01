package execution

import (
	"github.com/dapperlabs/flow-go/pkg/types"
)

type VirtualMachine interface {
	GetStorage()
	GetComputer()
	ExecuteTransaction(tx *types.Transaction, StartState StateCommitment) (ExecutedTransaction, error)
}
