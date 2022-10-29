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

	UnsafeRandomGenerator
	CryptoLibrary

	BlockInfo
	AccountInfo
	TransactionInfo

	ValueStore

	*SystemContracts

	UUIDGenerator

	AccountCreator
	AccountFreezer

	AccountKeyReader
	AccountKeyUpdater

	*ContractReader
	ContractUpdater
	*Programs

	accounts Accounts
	txnState *state.TransactionState
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
		txnState: txnState,
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
	env := newFacadeEnvironment(
		params,
		txnState,
		programs,
		NewCancellableMeter(ctx, txnState))

	env.addParseRestrictedChecks()

	return env
}

func newTransactionFacadeEnvironment(
	params EnvironmentParams,
	txnState *state.TransactionState,
	programs TransactionPrograms,
) *facadeEnvironment {
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

	env.addParseRestrictedChecks()

	return env
}

func (env *facadeEnvironment) addParseRestrictedChecks() {
	// NOTE: Cadence can access Programs, ContractReader, Meter and
	// ProgramLogger while it is parsing programs; all other access are
	// unexpected and are potentially program cache invalidation bugs.
	//
	// Also note that Tracer and SystemContracts are unguarded since these are
	// not accessible by Cadence.

	env.AccountCreator = NewParseRestrictedAccountCreator(
		env.txnState,
		env.AccountCreator)
	env.AccountFreezer = NewParseRestrictedAccountFreezer(
		env.txnState,
		env.AccountFreezer)
	env.AccountInfo = NewParseRestrictedAccountInfo(
		env.txnState,
		env.AccountInfo)
	env.AccountKeyReader = NewParseRestrictedAccountKeyReader(
		env.txnState,
		env.AccountKeyReader)
	env.AccountKeyUpdater = NewParseRestrictedAccountKeyUpdater(
		env.txnState,
		env.AccountKeyUpdater)
	env.BlockInfo = NewParseRestrictedBlockInfo(
		env.txnState,
		env.BlockInfo)
	env.ContractUpdater = NewParseRestrictedContractUpdater(
		env.txnState,
		env.ContractUpdater)
	env.CryptoLibrary = NewParseRestrictedCryptoLibrary(
		env.txnState,
		env.CryptoLibrary)
	env.EventEmitter = NewParseRestrictedEventEmitter(
		env.txnState,
		env.EventEmitter)
	env.TransactionInfo = NewParseRestrictedTransactionInfo(
		env.txnState,
		env.TransactionInfo)
	env.UnsafeRandomGenerator = NewParseRestrictedUnsafeRandomGenerator(
		env.txnState,
		env.UnsafeRandomGenerator)
	env.UUIDGenerator = NewParseRestrictedUUIDGenerator(
		env.txnState,
		env.UUIDGenerator)
	env.ValueStore = NewParseRestrictedValueStore(
		env.txnState,
		env.ValueStore)
}

func (env *facadeEnvironment) FlushPendingUpdates() (
	programs.OCCProgramsInvalidator,
	error,
) {
	keys, err := env.ContractUpdater.Commit()
	return programs.OCCProgramsInvalidator{
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
