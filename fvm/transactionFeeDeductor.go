package fvm

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/errors"
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

	txErr, fatalErr := d.deductFees(vm, ctx, proc, sth, programs)
	// TODO handle deduct fee failures, for now just return as error
	if txErr != nil {
		return txErr
	}
	return fatalErr
}

func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (errors.Error, error) {
	txErr, fatalErr := vm.invokeContractFunction(
		common.AddressLocation{
			Address: common.BytesToAddress(ctx.Chain.ServiceAddress().Bytes()),
			Name:    "FlowServiceAccount",
		},
		"deductTransactionFee",
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(proc.Transaction.Payer.Bytes())),
		},
		[]sema.Type{
			sema.AuthAccountType,
		},
		ctx,
		sth,
		programs,
	)

	return txErr, fatalErr
}
