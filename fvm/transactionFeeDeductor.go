package fvm

import (
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
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

	return d.deductFees(vm, ctx, proc, sth, programs)
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	feeTx := deductTransactionFeeTransaction(proc.Transaction.Payer, ctx.Chain.ServiceAddress())

	txErr := vm.invokeMetaTransaction(
		*ctx,
		feeTx,
		sth,
		programs,
	)
	if txErr == nil {
		proc.Events = append(proc.Events, feeTx.Events...)
		proc.ServiceEvents = append(proc.ServiceEvents, feeTx.ServiceEvents...)
		proc.Logs = append(proc.Logs, feeTx.Logs...)
		proc.GasUsed = proc.GasUsed + feeTx.GasUsed
	}

	return txErr
}
