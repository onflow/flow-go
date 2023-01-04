package fvm

import (
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(patrick): remove once https://github.com/onflow/flow-emulator/pull/242
// is integrated into flow-go's integration test.
func NewTransactionEnvironment(
	ctx Context,
	txnState *state.TransactionState,
	derivedTxnData environment.DerivedTransactionData,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) environment.Environment {
	ctx.Span = traceSpan
	ctx.TxIndex = txIndex
	ctx.TxId = tx.ID()
	ctx.TxBody = tx

	return environment.NewTransactionEnvironment(
		ctx.TracerSpan,
		ctx.EnvironmentParams,
		txnState,
		derivedTxnData)
}
