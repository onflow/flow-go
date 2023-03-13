package fvm

import (
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/model/flow"
)

func Transaction(
	txn *flow.TransactionBody,
	txnIndex uint32,
) *TransactionProcedure {
	return NewTransaction(txn.ID(), txnIndex, txn)
}

// TODO(patrick): pass in initial snapshot time when we start supporting
// speculative pre-processing / execution.
func NewTransaction(
	txnId flow.Identifier,
	txnIndex uint32,
	txnBody *flow.TransactionBody,
) *TransactionProcedure {
	return &TransactionProcedure{
		ID:                     txnId,
		Transaction:            txnBody,
		InitialSnapshotTxIndex: txnIndex,
		TxIndex:                txnIndex,
	}
}

type TransactionProcedure struct {
	ID                     flow.Identifier
	Transaction            *flow.TransactionBody
	InitialSnapshotTxIndex uint32
	TxIndex                uint32

	// TODO(patrick): remove
	ProcedureOutput
}

func (proc *TransactionProcedure) NewExecutor(
	ctx Context,
	txnState storage.Transaction,
) ProcedureExecutor {
	return newTransactionExecutor(ctx, proc, txnState)
}

func (proc *TransactionProcedure) SetOutput(output ProcedureOutput) {
	proc.ProcedureOutput = output
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

func (proc *TransactionProcedure) InitialSnapshotTime() derived.LogicalTime {
	return derived.LogicalTime(proc.InitialSnapshotTxIndex)
}

func (proc *TransactionProcedure) ExecutionTime() derived.LogicalTime {
	return derived.LogicalTime(proc.TxIndex)
}
