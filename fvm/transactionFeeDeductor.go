package fvm

import (
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionFeeDeductor struct{}

func NewTransactionFeeDeductor() *TransactionFeeDeductor {
	return &TransactionFeeDeductor{}
}

func (d *TransactionFeeDeductor) Process(
	vm *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	return d.deductFees(vm, ctx, proc.Transaction, sth, programs)
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx *Context,
	tx *flow.TransactionBody,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	return vm.invokeMetaTransaction(
		*ctx,
		deductTransactionFeeTransaction(tx.Payer, ctx.Chain.ServiceAddress()),
		sth,
		programs,
	)
}
