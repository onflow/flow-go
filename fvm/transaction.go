package fvm

import (
	"fmt"
	"runtime/debug"
	"strings"

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

	if proc.Transaction.Payer == ctx.Chain.ServiceAddress() {
		st.SetPayerIsServiceAccount()
	}

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

func (proc *TransactionProcedure) ComputationLimit(ctx Context) uint64 {
	// TODO for BFT (enforce max computation limit, already checked by collection nodes)
	// TODO replace tx.Gas with individual limits for computation and memory

	// decide computation limit
	computationLimit := proc.Transaction.GasLimit
	// if the computation limit is set to zero by user, fallback to the gas limit set by the context
	if computationLimit == 0 {
		computationLimit = ctx.ComputationLimit
		// if the context computation limit is also zero, fallback to the default computation limit
		if computationLimit == 0 {
			computationLimit = DefaultComputationLimit
		}
	}
	return computationLimit
}

func (proc *TransactionProcedure) MemoryLimit(ctx Context) uint64 {
	// TODO for BFT (enforce max computation limit, already checked by collection nodes)
	// TODO let user select a lower limit for memory (when its part of fees)

	memoryLimit := uint64(DefaultMemoryLimit) // TODO use the one set by tx
	// if the memory limit is set to zero by user, fallback to the gas limit set by the context
	if memoryLimit == 0 {
		memoryLimit = ctx.MemoryLimit
		// if the context memory limit is also zero, fallback to the default memory limit
		if memoryLimit == 0 {
			memoryLimit = DefaultMemoryLimit
		}
	}
	return memoryLimit
}
