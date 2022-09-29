package environment

import (
	"context"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

var _ Environment = &facadeEnvironment{}

// facadeEnvironment exposes various fvm business logic as a single interface.
type facadeEnvironment struct {
	*Runtime

	*Tracer
	Meter

	*ProgramLogger
	EventEmitter

	*UnsafeRandomGenerator
	*CryptoLibrary

	*BlockInfo
	*AccountInfo
	TransactionInfo

	*ValueStore

	*SystemContracts

	*UUIDGenerator

	AccountCreator
	AccountFreezer

	*AccountKeyReader
	AccountKeyUpdater

	*ContractReader
	ContractUpdater
	*Programs

	accounts Accounts
}

func newFacadeEnvironment(
	params EnvironmentParams,
	txnState *state.TransactionState,
	programs TransactionPrograms,
	meter Meter,
) *facadeEnvironment {
	tracer := NewTracer(params.TracerParams)
	accounts := NewAccounts(txnState)
	logger := NewProgramLogger(tracer, params.ProgramLoggerParams)
	runtime := NewRuntime(params.RuntimeParams)
	systemContracts := NewSystemContracts(
		params.Chain,
		tracer,
		logger,
		runtime)

	env := &facadeEnvironment{
		Runtime: runtime,

		Tracer: tracer,
		Meter:  meter,

		ProgramLogger: logger,
		EventEmitter:  NoEventEmitter{},

		UnsafeRandomGenerator: NewUnsafeRandomGenerator(
			tracer,
			params.BlockHeader,
		),
		CryptoLibrary: NewCryptoLibrary(tracer, meter),

		BlockInfo: NewBlockInfo(
			tracer,
			meter,
			params.BlockHeader,
			params.Blocks,
		),
		AccountInfo: NewAccountInfo(
			tracer,
			meter,
			accounts,
			systemContracts,
			params.ServiceAccountEnabled,
		),
		TransactionInfo: NoTransactionInfo{},

		ValueStore: NewValueStore(
			tracer,
			meter,
			accounts,
		),

		SystemContracts: systemContracts,

		UUIDGenerator: NewUUIDGenerator(
			tracer,
			meter,
			txnState),

		AccountCreator: NoAccountCreator{},
		AccountFreezer: NoAccountFreezer{},

		AccountKeyReader: NewAccountKeyReader(
			tracer,
			meter,
			accounts,
		),
		AccountKeyUpdater: NoAccountKeyUpdater{},

		ContractReader: NewContractReader(
			tracer,
			meter,
			accounts,
		),
		ContractUpdater: NoContractUpdater{},
		Programs: NewPrograms(
			tracer,
			meter,
			txnState,
			accounts,
			programs),

		accounts: accounts,
	}

	env.Runtime.SetEnvironment(env)

	return env
}

func newScriptFacadeEnvironment(
	ctx context.Context,
	params EnvironmentParams,
	txnState *state.TransactionState,
	programs TransactionPrograms,
) *facadeEnvironment {
	return newFacadeEnvironment(
		params,
		txnState,
		programs,
		NewCancellableMeter(ctx, txnState))
}

func newTransactionFacadeEnvironment(
	params EnvironmentParams,
	txnState *state.TransactionState,
	programs TransactionPrograms,
) Environment {
	env := newFacadeEnvironment(
		params,
		txnState,
		programs,
		NewMeter(txnState),
	)

	env.TransactionInfo = NewTransactionInfo(
		params.TransactionInfoParams,
		env.Tracer,
		params.Chain.ServiceAddress(),
	)
	env.EventEmitter = NewEventEmitter(
		env.Tracer,
		env.Meter,
		params.Chain,
		params.TransactionInfoParams,
		params.EventEmitterParams,
	)
	env.AccountCreator = NewAccountCreator(
		txnState,
		params.Chain,
		env.accounts,
		params.ServiceAccountEnabled,
		env.Tracer,
		env.Meter,
		params.MetricsReporter,
		env.SystemContracts)
	env.AccountFreezer = NewAccountFreezer(
		params.Chain.ServiceAddress(),
		env.accounts,
		env.TransactionInfo)
	env.ContractUpdater = NewContractUpdater(
		env.Tracer,
		env.Meter,
		env.accounts,
		env.TransactionInfo,
		params.Chain,
		params.ContractUpdaterParams,
		env.ProgramLogger,
		env.SystemContracts,
		env.Runtime)

	env.AccountKeyUpdater = NewAccountKeyUpdater(
		env.Tracer,
		env.Meter,
		env.accounts,
		txnState,
		env)

	return env
}

func (env *facadeEnvironment) FlushPendingUpdates() (
	programs.ModifiedSetsInvalidator,
	error,
) {
	keys, err := env.ContractUpdater.Commit()
	return programs.ModifiedSetsInvalidator{
		ContractUpdateKeys: keys,
		FrozenAccounts:     env.FrozenAccounts(),
	}, err
}

func (env *facadeEnvironment) Reset() {
	env.ContractUpdater.Reset()
	env.EventEmitter.Reset()
	env.AccountFreezer.Reset()
}

// Miscellaneous cadence runtime.Interface API.
func (facadeEnvironment) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}
