package fvm

import (
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(patrick): rm after emulator is updated
type Environment = environment.Environment

// DEPRECATED.  DO NOT USE
//
// TODO(patrick): rm after updating emulator
func NewTransactionEnvironment(
	ctx Context,
	vm *VirtualMachine,
	txnState *state.TransactionState,
	programs environment.TransactionPrograms,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) environment.Environment {
	return NewTransactionEnv(
		ctx,
		txnState,
		programs,
		tx,
		txIndex,
		traceSpan)
}

func NewTransactionEnv(
	ctx Context,
	txnState *state.TransactionState,
	programs environment.TransactionPrograms,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) environment.Environment {
	ctx.RootSpan = traceSpan
	ctx.TxIndex = txIndex
	ctx.TxId = tx.ID()
	ctx.TxBody = tx

	return environment.NewTransactionEnvironment(
		ctx.EnvironmentParams,
		txnState,
		programs)
}
