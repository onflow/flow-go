package fvm

import (
	"github.com/opentracing/opentracing-go"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody, txIndex uint32) *TransactionProcedure {
	return &TransactionProcedure{
		ID:          tx.ID(),
		Transaction: tx,
		TxIndex:     txIndex,
	}
}

type TransactionProcessor interface {
	Process(*VirtualMachine, *Context, *TransactionProcedure, *state.StateHolder, *programs.Programs) (txError errors.TransactionError, vmError errors.VMError)
}

type TransactionProcedure struct {
	ID            flow.Identifier
	Transaction   *flow.TransactionBody
	TxIndex       uint32
	Logs          []string
	Events        []flow.Event
	ServiceEvents []flow.Event
	GasUsed       uint64
	Err           errors.TransactionError
	Retried       int
	TraceSpan     opentracing.Span
}

func (proc *TransactionProcedure) SetTraceSpan(traceSpan opentracing.Span) {
	proc.TraceSpan = traceSpan
}

func (proc *TransactionProcedure) Run(vm *VirtualMachine, ctx Context, st *state.StateHolder, programs *programs.Programs) error {

	for _, p := range ctx.TransactionProcessors {
		txErr, vmErr := p.Process(vm, &ctx, proc, st, programs)
		if vmErr != nil {
			return vmErr
		}

		if txErr != nil {
			proc.Err = txErr
			// TODO we should not break here we should continue for fee deductions
			break
		}
	}

	return nil
}
