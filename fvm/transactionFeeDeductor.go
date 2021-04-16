package fvm

import (
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
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
	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMDeductTransactionFees)
		defer span.Finish()
	}

	txErr, fatalErr := d.deductFees(vm, ctx, proc.Transaction, sth, programs)
	// TODO handle deduct fee failures, for now just return as error
	if txErr != nil {
		return txErr
	}
	return fatalErr
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx *Context,
	tx *flow.TransactionBody,
	sth *state.StateHolder,
	programs *programs.Programs,
) (errors.Error, error) {
	return vm.invokeMetaTransaction(
		*ctx,
		deductTransactionFeeTransaction(tx.Payer, ctx.Chain.ServiceAddress()),
		sth,
		programs,
	)
}
