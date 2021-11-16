package fvm

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"runtime/debug"
	"strings"

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
	Process(*VirtualMachine, *Context, *TransactionProcedure, *state.StateHolder, *programs.Programs) error
}

type TransactionProcedure struct {
	ID              flow.Identifier
	Transaction     *flow.TransactionBody
	TxIndex         uint32
	Logs            []string
	Events          []flow.Event
	ServiceEvents   []flow.Event
	ComputationUsed uint64
	Err             errors.Error
	Retried         int
	TraceSpan       opentracing.Span
}

func (proc *TransactionProcedure) SetTraceSpan(traceSpan opentracing.Span) {
	proc.TraceSpan = traceSpan
}

func (proc *TransactionProcedure) Run(vm *VirtualMachine, ctx Context, st *state.StateHolder, programs *programs.Programs) error {

	defer func() {
		if r := recover(); r != nil {

			if strings.Contains(fmt.Sprintf("%v", r), "[Error Code: 1106]") {
				ctx.Logger.Error().Str("trace", string(debug.Stack())).Msg("VM LedgerIntractionLimitExceeded panic")
				proc.Err = errors.NewLedgerIntractionLimitExceededError(state.DefaultMaxInteractionSize, state.DefaultMaxInteractionSize)
				return
			}

			panic(r)
		}
	}()

	for _, p := range ctx.TransactionProcessors {
		err := p.Process(vm, &ctx, proc, st, programs)
		txErr, failure := errors.SplitErrorTypes(err)
		if failure != nil {
			// log the full error path
			ctx.Logger.Err(err).Msg("fatal error when execution a transaction")
			return failure
		}

		if txErr != nil {
			proc.Err = txErr
			// TODO we should not break here we should continue for fee deductions
			break
		}
	}

	return nil
}
