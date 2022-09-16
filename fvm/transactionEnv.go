package fvm

import (
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/handler"
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
	programs handler.TransactionPrograms,
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
	programs handler.TransactionPrograms,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) environment.Environment {
	txID := tx.ID()
	// TODO set the flags on context

	env := newFacadeEnvironment(
		ctx,
		sth,
		programs,
		environment.NewTracer(ctx.Tracer, traceSpan, ctx.ExtensiveTracing),
		environment.NewMeter(sth),
	)

	env.TransactionInfo = environment.NewTransactionInfo(
		txIndex,
		txID,
		ctx.TransactionFeesEnabled,
		ctx.LimitAccountStorage,
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
		ctx.ServiceEventCollectionEnabled,
		ctx.EventCollectionByteSizeLimit,
	)
	env.AccountCreator = environment.NewAccountCreator(
		sth,
		ctx.Chain,
		env.accounts,
		ctx.ServiceAccountEnabled,
		env.Tracer,
		env.Meter,
		ctx.Metrics,
		env.SystemContracts)
	env.AccountFreezer = environment.NewAccountFreezer(
		ctx.Chain.ServiceAddress(),
		env.accounts,
		env.TransactionInfo)
	env.ContractUpdater = handler.NewContractUpdater(
		env.Tracer,
		env.Meter,
		env.accounts,
		env.TransactionInfo,
		ctx.Chain,
		ctx.RestrictContractDeployment,
		ctx.RestrictContractRemoval,
		env.ProgramLogger,
		env.SystemContracts,
		env.Runtime)

	env.AccountKeyUpdater = handler.NewAccountKeyUpdater(
		env.Tracer,
		env.Meter,
		env.accounts,
		sth,
		env)

	return env
}
