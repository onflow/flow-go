package execution

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

type VirtualMachine interface {
	GetStorage()
	GetComputer()
	ExecuteTransaction(tx *flow.Transaction, StartState storage.StateCommitment) (ExecutedTransaction, error)
}
