package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/model/flow"
)

// Environment implements the accounts business logic and exposes cadence
// runtime interface methods for the runtime.
type Environment interface {
	runtime.Interface

	Tracer

	Meter

	MetricsReporter

	// Runtime
	BorrowCadenceRuntime() ReusableCadenceRuntime
	ReturnCadenceRuntime(ReusableCadenceRuntime)

	TransactionInfo

	// ProgramLogger
	LoggerProvider
	Logs() []string

	// EventEmitter
	Events() flow.EventsList
	ServiceEvents() flow.EventsList
	ConvertedServiceEvents() flow.ServiceEventList

	// SystemContracts
	ContractFunctionInvoker

	AccountsStorageCapacity(
		addresses []flow.Address,
		payer flow.Address,
		maxTxFees uint64,
	) (
		cadence.Value,
		error,
	)
	CheckPayerBalanceAndGetMaxTxFees(
		payer flow.Address,
		inclusionEffort uint64,
		executionEffort uint64,
	) (
		cadence.Value,
		error,
	)
	DeductTransactionFees(
		payer flow.Address,
		inclusionEffort uint64,
		executionEffort uint64,
	) (
		cadence.Value,
		error,
	)

	// AccountInfo
	GetAccount(address flow.Address) (*flow.Account, error)
	GetAccountKeys(address flow.Address) ([]flow.AccountPublicKey, error)

	// RandomSourceHistory is the current block's derived random source.
	// This source is only used by the core-contract that tracks the random source
	// history for commit-reveal schemes.
	RandomSourceHistory() ([]byte, error)

	// FlushPendingUpdates flushes pending updates from the stateful environment
	// modules (i.e., ContractUpdater) to the state transaction, and return
	// the updated contract keys.
	FlushPendingUpdates() (
		ContractUpdates,
		error,
	)

	// Reset resets all stateful environment modules (e.g., ContractUpdater,
	// EventEmitter) to initial state.
	Reset()
}

// ReusableCadenceRuntime is a wrapper around the cadence runtime and environment that
// is reused between procedures
type ReusableCadenceRuntime interface {
	// ReadStored calls the internal runtime.Runtime.ReadStored
	ReadStored(
		address common.Address,
		path cadence.Path,
	) (
		cadence.Value,
		error,
	)

	// NewTransactionExecutor calls the internal runtime.Runtime.NewTransactionExecutor
	NewTransactionExecutor(
		script runtime.Script,
		location common.Location,
	) runtime.Executor

	// ExecuteScript calls the internal runtime.Runtime.ExecuteScript
	ExecuteScript(
		script runtime.Script,
		location common.Location,
	) (
		cadence.Value,
		error,
	)

	// InvokeContractFunction calls the internal runtime.Runtime.InvokeContractFunction
	InvokeContractFunction(
		contractLocation common.AddressLocation,
		functionName string,
		arguments []cadence.Value,
		argumentTypes []sema.Type,
	) (
		cadence.Value,
		error,
	)

	// CadenceTXEnv returns a cadence runtime.Environment set up for use in transactions
	CadenceTXEnv() runtime.Environment

	// CadenceScriptEnv returns a cadence runtime.Environment set up for use in scripts
	CadenceScriptEnv() runtime.Environment

	// SetFvmEnvironment sets the underlying Environment for the cadence runtime to use
	SetFvmEnvironment(fvmEnv Environment)
}

type EnvironmentParams struct {
	Chain flow.Chain

	// NOTE: The ServiceAccountEnabled option is used by the playground
	// https://github.com/onflow/flow-playground-api/blob/1ad967055f31db8f1ce88e008960e5fc14a9fbd1/compute/computer.go#L76
	ServiceAccountEnabled bool

	RuntimeParams

	ProgramLoggerParams

	EventEmitterParams

	BlockInfoParams
	TransactionInfoParams
	ScriptInfoParams

	EntropyProvider
	ExecutionVersionProvider

	ContractUpdaterParams
}

func (env *EnvironmentParams) SetScriptInfoParams(info *ScriptInfoParams) {
	env.ScriptInfoParams = *info
}
