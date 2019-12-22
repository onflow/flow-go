package flow

type VirtualMachine interface {
	GetStorage()
	GetComputer()
	ExecuteTransaction(tx *Transaction, StartState StateCommitment) (ExecutedTransaction, error)
}
