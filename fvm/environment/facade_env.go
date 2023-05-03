package environment

import (
	"context"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
)

var _ Environment = &facadeEnvironment{}

// facadeEnvironment exposes various fvm business logic as a single interface.
type facadeEnvironment struct {
	*Runtime

	tracing.TracerSpan
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

	AccountKeyReader
	AccountKeyUpdater

	*ContractReader
	ContractUpdater
	*Programs

	accounts Accounts
	txnState storage.TransactionPreparer
}

func newFacadeEnvironment(
	tracer tracing.TracerSpan,
	params EnvironmentParams,
	txnState storage.TransactionPreparer,
	meter Meter,
) *facadeEnvironment {
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

		TracerSpan: tracer,
		Meter:      meter,

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
			params.MetricsReporter,
			txnState,
			accounts),

		accounts: accounts,
		txnState: txnState,
	}

	env.Runtime.SetEnvironment(env)

	return env
}

// This is mainly used by command line tools, the emulator, and cadence tools
// testing.
func NewScriptEnvironmentFromStorageSnapshot(
	params EnvironmentParams,
	storageSnapshot snapshot.StorageSnapshot,
) *facadeEnvironment {
	derivedBlockData := derived.NewEmptyDerivedBlockData(0)
	derivedTxn := derivedBlockData.NewSnapshotReadDerivedTransactionData()

	txn := storage.SerialTransaction{
		NestedTransactionPreparer: state.NewTransactionState(
			storageSnapshot,
			state.DefaultParameters()),
		DerivedTransactionData: derivedTxn,
	}

	return NewScriptEnv(
		context.Background(),
		tracing.NewTracerSpan(),
		params,
		txn)
}

func NewScriptEnv(
	ctx context.Context,
	tracer tracing.TracerSpan,
	params EnvironmentParams,
	txnState storage.TransactionPreparer,
) *facadeEnvironment {
	env := newFacadeEnvironment(
		tracer,
		params,
		txnState,
		NewCancellableMeter(ctx, txnState))

	env.addParseRestrictedChecks()

	return env
}

func NewTransactionEnvironment(
	tracer tracing.TracerSpan,
	params EnvironmentParams,
	txnState storage.TransactionPreparer,
) *facadeEnvironment {
	env := newFacadeEnvironment(
		tracer,
		params,
		txnState,
		NewMeter(txnState),
	)

	env.TransactionInfo = NewTransactionInfo(
		params.TransactionInfoParams,
		tracer,
		params.Chain.ServiceAddress(),
	)
	env.EventEmitter = NewEventEmitter(
		tracer,
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
		tracer,
		env.Meter,
		params.MetricsReporter,
		env.SystemContracts)
	env.ContractUpdater = NewContractUpdater(
		tracer,
		env.Meter,
		env.accounts,
		params.TransactionInfoParams.TxBody.Authorizers,
		params.Chain,
		params.ContractUpdaterParams,
		env.ProgramLogger,
		env.SystemContracts,
		env.Runtime)

	env.AccountKeyUpdater = NewAccountKeyUpdater(
		tracer,
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
	ContractUpdates,
	error,
) {
	return env.ContractUpdater.Commit()
}

func (env *facadeEnvironment) Reset() {
	env.ContractUpdater.Reset()
	env.EventEmitter.Reset()
	env.Programs.Reset()
}

// Miscellaneous cadence runtime.Interface API.
func (facadeEnvironment) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}

func (env *facadeEnvironment) SetInterpreterSharedState(state *interpreter.SharedState) {
	// NO-OP
}

func (env *facadeEnvironment) GetInterpreterSharedState() *interpreter.SharedState {
	return nil
}
