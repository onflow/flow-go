package fvm

import (
	otelTrace "go.opentelemetry.io/otel/trace"

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
	// Process processes a transaction.
	// Process should expect that the transaction procedure might already contain an error.
	// Process can only return a Failure in which case execution will halt.
	Process(*VirtualMachine, Context, *TransactionProcedure, *state.StateHolder, *programs.Programs) errors.Failure
}

type TransactionProcedure struct {
	ID              flow.Identifier
	Transaction     *flow.TransactionBody
	TxIndex         uint32
	Logs            []string
	Events          []flow.Event
	ServiceEvents   []flow.Event
	ComputationUsed uint64
	MemoryEstimate  uint64
	Err             errors.Error
	TraceSpan       otelTrace.Span
}

func (proc *TransactionProcedure) SetTraceSpan(traceSpan otelTrace.Span) {
	proc.TraceSpan = traceSpan
}

// Run runs the transaction procedure.
// returning an error indicates a critical failure in the FVM. No other transactions should be executed afterwards.
func (proc *TransactionProcedure) Run(vm *VirtualMachine, ctx Context, st *state.StateHolder, programs *programs.Programs) error {
	for _, p := range ctx.TransactionProcessors {
		failure := p.Process(vm, ctx, proc, st, programs)
		if failure != nil {
			ctx.Logger.
				Err(failure).
				Uint16("failure code", uint16(failure.FailureCode())).
				Msg("fatal error when execution a transaction")
			return failure
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
	// TODO for BFT (enforce max memory limit, already checked by collection nodes)
	// TODO let user select a lower limit for memory (when its part of fees)

	memoryLimit := ctx.MemoryLimit // TODO use the one set by tx
	// if the context memory limit is also zero, fallback to the default memory limit
	if memoryLimit == 0 {
		memoryLimit = DefaultMemoryLimit
	}
	return memoryLimit
}

func (proc *TransactionProcedure) ShouldDisableMemoryAndInteractionLimits(
	ctx Context,
) bool {
	return ctx.DisableMemoryAndInteractionLimits ||
		proc.Transaction.Payer == ctx.Chain.ServiceAddress()
}
