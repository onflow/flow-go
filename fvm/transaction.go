package fvm

import (
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(patrick): pass in initial snapshot time when we start supporting
// speculative pre-processing / execution.
func Transaction(tx *flow.TransactionBody, txIndex uint32) *TransactionProcedure {
	return &TransactionProcedure{
		ID:                     tx.ID(),
		Transaction:            tx,
		InitialSnapshotTxIndex: txIndex,
		TxIndex:                txIndex,
	}
}

type TransactionProcessor interface {
	Process(
		Context,
		*TransactionProcedure,
		*state.StateHolder,
		*programs.TransactionPrograms,
	) error
}

type TransactionProcedure struct {
	ID                     flow.Identifier
	Transaction            *flow.TransactionBody
	InitialSnapshotTxIndex uint32
	TxIndex                uint32

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

func (proc *TransactionProcedure) Run(
	ctx Context,
	st *state.StateHolder,
	programs *programs.TransactionPrograms,
) error {
	for _, p := range ctx.TransactionProcessors {
		err := p.Process(ctx, proc, st, programs)
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

func (TransactionProcedure) Type() ProcedureType {
	return TransactionProcedureType
}

func (proc *TransactionProcedure) InitialSnapshotTime() programs.LogicalTime {
	return programs.LogicalTime(proc.InitialSnapshotTxIndex)
}

func (proc *TransactionProcedure) ExecutionTime() programs.LogicalTime {
	return programs.LogicalTime(proc.TxIndex)
}
