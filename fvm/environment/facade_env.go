package environment

import (
	"context"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
)

var _ Environment = &facadeEnvironment{}

// facadeEnvironment exposes various fvm business logic as a single interface.
type facadeEnvironment struct {
	CadenceRuntimeProvider

	tracing.TracerSpan
	Meter

	*ProgramLogger
	EventEmitter

	RandomGenerator
	CryptoLibrary
	RandomSourceHistoryProvider

	BlockInfo
	AccountInfo
	TransactionInfo

	ValueStore

	*SystemContracts
	MinimumCadenceRequiredVersion

	UUIDGenerator
	AccountLocalIDGenerator

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
	runtime CadenceRuntimeProvider,
) *facadeEnvironment {
	accounts := NewAccounts(txnState)
	logger := NewProgramLogger(tracer, params.ProgramLoggerParams)
	chain := params.Chain
	systemContracts := NewSystemContracts(
		chain,
		tracer,
		logger,
		runtime)

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	env := &facadeEnvironment{
		CadenceRuntimeProvider: runtime,

		TracerSpan: tracer,
		Meter:      meter,

		ProgramLogger: logger,
		EventEmitter:  NoEventEmitter{},

		CryptoLibrary:               NewCryptoLibrary(tracer, meter),
		RandomSourceHistoryProvider: NewForbiddenRandomSourceHistoryProvider(),

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
		MinimumCadenceRequiredVersion: NewMinimumCadenceRequiredVersion(
			params.ExecutionVersionProvider,
		),

		UUIDGenerator: NewUUIDGenerator(
			tracer,
			params.Logger,
			meter,
			txnState,
			params.BlockHeader,
			params.TxIndex),
		AccountLocalIDGenerator: NewAccountLocalIDGenerator(
			tracer,
			meter,
			accounts,
		),

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
			common.Address(sc.Crypto.Address),
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

	env.CadenceRuntimeProvider.SetEnvironment(env)

	return env
}

// This is mainly used by command line tools, the emulator, and cadence tools
// testing.
func NewScriptEnvironmentFromStorageSnapshot(
	params EnvironmentParams,
	storageSnapshot snapshot.StorageSnapshot,
) *facadeEnvironment {
	blockDatabase := storage.NewBlockDatabase(storageSnapshot, 0, nil)

	return NewScriptEnv(
		context.Background(),
		tracing.NewTracerSpan(),
		params,
		blockDatabase.NewSnapshotReadTransaction(state.DefaultParameters()))
}

func NewScriptEnv(
	ctx context.Context,
	tracer tracing.TracerSpan,
	params EnvironmentParams,
	txnState storage.TransactionPreparer,
) *facadeEnvironment {
	cadenceRuntime := NewRuntime(params.RuntimeParams, CadenceScriptRuntime)

	env := newFacadeEnvironment(
		tracer,
		params,
		txnState,
		NewCancellableMeter(ctx, txnState),
		cadenceRuntime)
	env.RandomGenerator = NewRandomGenerator(
		tracer,
		params.EntropyProvider,
		params.ScriptInfoParams.ID[:],
	)
	env.addParseRestrictedChecks()
	return env
}

func NewTransactionEnvironment(
	tracer tracing.TracerSpan,
	params EnvironmentParams,
	txnState storage.TransactionPreparer,
) *facadeEnvironment {
	cadenceRuntime := NewRuntime(params.RuntimeParams, CadenceTransactionRuntime)
	env := newFacadeEnvironment(
		tracer,
		params,
		txnState,
		NewMeter(txnState),
		cadenceRuntime,
	)

	env.TransactionInfo = NewTransactionInfo(
		params.TransactionInfoParams,
		tracer,
		params.Chain.ServiceAddress(),
	)
	env.EventEmitter = NewEventEmitter(
		env.Logger(),
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
		cadenceRuntime)

	env.AccountKeyUpdater = NewAccountKeyUpdater(
		tracer,
		env.Meter,
		env.accounts,
		txnState,
		env)

	env.RandomGenerator = NewRandomGenerator(
		tracer,
		params.EntropyProvider,
		params.TxId[:],
	)

	env.RandomSourceHistoryProvider = NewRandomSourceHistoryProvider(
		tracer,
		env.Meter,
		params.EntropyProvider,
		params.TransactionInfoParams.RandomSourceHistoryCallAllowed,
	)

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
	env.RandomGenerator = NewParseRestrictedRandomGenerator(
		env.txnState,
		env.RandomGenerator)
	env.RandomSourceHistoryProvider = NewParseRestrictedRandomSourceHistoryProvider(
		env.txnState,
		env.RandomSourceHistoryProvider)
	env.UUIDGenerator = NewParseRestrictedUUIDGenerator(
		env.txnState,
		env.UUIDGenerator)
	env.AccountLocalIDGenerator = NewParseRestrictedAccountLocalIDGenerator(
		env.txnState,
		env.AccountLocalIDGenerator)
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

// Miscellaneous Cadence runtime.Interface API

func (*facadeEnvironment) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
	// NO-OP
}

func (env *facadeEnvironment) RecoverProgram(program *ast.Program, location common.Location) ([]byte, error) {
	return RecoverProgram(
		env.chain.ChainID(),
		program,
		location,
	)
}

func (env *facadeEnvironment) ValidateAccountCapabilitiesGet(
	_ interpreter.AccountCapabilityGetValidationContext,
	_ interpreter.AddressValue,
	_ interpreter.PathValue,
	wantedBorrowType *sema.ReferenceType,
	_ *sema.ReferenceType,
) (bool, error) {
	_, hasEntitlements := wantedBorrowType.Authorization.(sema.EntitlementSetAccess)
	if hasEntitlements {
		// TODO: maybe abort
		//return false, &interpreter.GetCapabilityError{}
		return false, nil
	}
	return true, nil
}

func (env *facadeEnvironment) ValidateAccountCapabilitiesPublish(
	_ interpreter.AccountCapabilityPublishValidationContext,
	_ interpreter.AddressValue,
	_ interpreter.PathValue,
	capabilityBorrowType *interpreter.ReferenceStaticType,
) (bool, error) {
	_, isEntitledCapability := capabilityBorrowType.Authorization.(interpreter.EntitlementSetAuthorization)
	if isEntitledCapability {
		// TODO: maybe abort
		return false, nil
	}
	return true, nil
}
