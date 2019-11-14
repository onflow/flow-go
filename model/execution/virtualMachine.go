package execution

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type VirtualMachine interface {
	GetStorage()
	GetComputer()
	ExecuteTransaction(tx *flow.Transaction, StartState StateCommitment) (ExecutedTransaction, error)
}
