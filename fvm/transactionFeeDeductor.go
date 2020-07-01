package fvm

import "github.com/dapperlabs/flow-go/model/flow"

type TransactionFeeDeductor struct{}

func NewTransactionFeeDeductor() *TransactionFeeDeductor {
	return &TransactionFeeDeductor{}
}

func (d *TransactionFeeDeductor) Process(
	vm *VirtualMachine,
	ctx Context,
	inv *InvokableTransaction,
	ledger Ledger,
) error {
	return d.deductFees(vm, ctx, inv.Transaction, ledger)
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx Context,
	tx *flow.TransactionBody,
	ledger Ledger,
) error {
	return vm.invokeMetaTransaction(
		ctx,
		deductTransactionFeeTransaction(tx.Payer, vm.chain.ServiceAddress()),
		ledger,
	)
}
