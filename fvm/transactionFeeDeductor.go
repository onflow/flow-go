package fvm

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionFeeDeductor struct{}

func NewTransactionFeeDeductor() *TransactionFeeDeductor {
	return &TransactionFeeDeductor{}
}

func (d *TransactionFeeDeductor) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *TransactionProcedure,
	stm *state.StateManager,
	programs *Programs,
) error {
	return d.deductFees(vm, ctx, proc.Transaction, stm, programs)
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx Context,
	tx *flow.TransactionBody,
	stm *state.StateManager,
	programs *Programs,
) error {
	if !ctx.TransactionFeesEnabled {
		return nil
	}

	return vm.invokeMetaTransaction(
		ctx,
		deductTransactionFeeTransaction(tx.Payer, ctx.Chain.ServiceAddress(), DefaultTransactionFees.ToGoValue().(uint64)),
		stm,
		programs,
	)
}
