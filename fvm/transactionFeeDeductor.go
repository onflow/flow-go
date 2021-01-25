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
	st *state.State,
) error {
	err := d.deductFees(vm, ctx, proc.Transaction, st)
	if err != nil {
		st.Commit()
	} else {
		st.Rollback()
	}
	return err
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx Context,
	tx *flow.TransactionBody,
	st *state.State,
) error {

	return vm.invokeMetaTransaction(
		ctx,
		deductTransactionFeeTransaction(tx.Payer, ctx.Chain.ServiceAddress()),
		st,
	)
}
