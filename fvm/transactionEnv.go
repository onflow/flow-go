package fvm

import (
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// DEPRECATED.  DO NOT USE
//
// TODO(patrick): rm after updating emulator
func NewTransactionEnvironment(
	ctx Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs environment.TransactionPrograms,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) environment.Environment {
	return NewTransactionEnv(
		ctx,
		sth,
		programs,
		tx,
		txIndex,
		traceSpan)
}

func NewTransactionEnv(
	ctx Context,
	sth *state.StateHolder,
	programs environment.TransactionPrograms,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) environment.Environment {
	txID := tx.ID()
	// TODO set the flags on context

	ctx.RootSpan = traceSpan
	env := newFacadeEnvironment(
		ctx,
		sth,
		programs,
		environment.NewTracer(ctx.TracerParams),
		environment.NewMeter(sth),
	)

	ctx.TxIndex = txIndex
	ctx.TxId = txID
	env.TransactionInfo = environment.NewTransactionInfo(
		ctx.TransactionInfoParams,
		env.Tracer,
		tx.Authorizers,
		ctx.Chain.ServiceAddress(),
	)
	env.EventEmitter = environment.NewEventEmitter(
		env.Tracer,
		env.Meter,
		ctx.Chain,
		txID,
		txIndex,
		tx.Payer,
		ctx.EventEmitterParams,
	)
	env.AccountCreator = environment.NewAccountCreator(
		sth,
		ctx.Chain,
		env.accounts,
		ctx.ServiceAccountEnabled,
		env.Tracer,
		env.Meter,
		ctx.MetricsReporter,
		env.SystemContracts)
	env.AccountFreezer = environment.NewAccountFreezer(
		ctx.Chain.ServiceAddress(),
		env.accounts,
		env.TransactionInfo)
	env.ContractUpdater = environment.NewContractUpdater(
		env.Tracer,
		env.Meter,
		env.accounts,
		env.TransactionInfo,
		ctx.Chain,
		ctx.ContractUpdaterParams,
		env.ProgramLogger,
		env.SystemContracts,
		env.Runtime)

	env.AccountKeyUpdater = environment.NewAccountKeyUpdater(
		env.Tracer,
		env.Meter,
		env.accounts,
		sth,
		env)

	return env
}
