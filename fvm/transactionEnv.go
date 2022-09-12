package fvm

import (
	"github.com/onflow/cadence/runtime"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

var _ runtime.Interface = &TransactionEnv{}

// TransactionEnv is a read-write environment used for executing flow transactions.
type TransactionEnv struct {
	commonEnv

	tx      *flow.TransactionBody
	txIndex uint32
	txID    flow.Identifier
}

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
) *TransactionEnv {
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
) *TransactionEnv {
	txID := tx.ID()
	// TODO set the flags on context
	tracer := environment.NewTracer(ctx.Tracer, traceSpan, ctx.ExtensiveTracing)
	meter := environment.NewMeter(sth)

	env := &TransactionEnv{
		commonEnv: newCommonEnv(
			ctx,
			sth,
			programs,
			tracer,
			meter,
		),
		tx:      tx,
		txIndex: txIndex,
		txID:    txID,
	}

	env.TransactionInfo = environment.NewTransactionInfo(
		tracer,
		tx.Authorizers,
		ctx.Chain.ServiceAddress(),
	)
	env.EventEmitter = environment.NewEventEmitter(
		tracer,
		meter,
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
		tracer,
		meter,
		ctx.Metrics,
		env.SystemContracts)
	env.AccountFreezer = environment.NewAccountFreezer(
		ctx.Chain.ServiceAddress(),
		env.accounts,
		env.TransactionInfo)
	env.ContractUpdater = handler.NewContractUpdater(
		tracer,
		meter,
		env.accounts,
		env.TransactionInfo,
		ctx.Chain,
		ctx.RestrictContractDeployment,
		ctx.RestrictContractRemoval,
		env.ProgramLogger,
		env.SystemContracts,
		env.Runtime)

	env.AccountKeyUpdater = handler.NewAccountKeyUpdater(
		tracer,
		meter,
		env.accounts,
		sth,
		env)

	env.Runtime.SetEnvironment(env)

	// TODO(patrick): rm this hack
	env.fullEnv = env

	return env
}

func (e *TransactionEnv) TxIndex() uint32 {
	return e.txIndex
}

func (e *TransactionEnv) TxID() flow.Identifier {
	return e.txID
}
