package processors

import (
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/opentracing/opentracing-go"
)

type FeeDeductor struct{}

func (FeeDeductor) DeductFees(
	vm context.VirtualMachine,
	ctx *context.Context,
	sth *state.StateHolder,
	programs *programs.Programs,
	payer flow.Address,
	parentSpan opentracing.Span,
) error {
	if parentSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(parentSpan, trace.FVMDeductTransactionFees)
		defer span.Finish()
	}

	feeTx := blueprints.DeductTransactionFeeTransaction(payer, ctx.Chain.ServiceAddress())
	txErr, fatalErr := vm.InvokeMetaTransaction(
		*ctx,
		feeTx,
		sth,
		programs,
	)

	// // TODO RAMTIN, capture these through envs passed to the InvokeMetaTransaction
	// if txErr == nil {
	// 	proc.Events = append(proc.Events, feeTx.Events...)
	// 	proc.ServiceEvents = append(proc.ServiceEvents, feeTx.ServiceEvents...)
	// 	proc.Logs = append(proc.Logs, feeTx.Logs...)
	// 	proc.GasUsed = proc.GasUsed + feeTx.GasUsed
	// }
	// TODO handle deduct fee failures, for now just return as error
	if txErr != nil {
		return txErr
	}

	return fatalErr
}
