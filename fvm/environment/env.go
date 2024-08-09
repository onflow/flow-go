package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
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
	BorrowCadenceRuntime() *reusableRuntime.ReusableCadenceRuntime
	ReturnCadenceRuntime(*reusableRuntime.ReusableCadenceRuntime)

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

	ContractUpdaterParams
}

func DefaultEnvironmentParams() EnvironmentParams {
	return EnvironmentParams{
		Chain:                 flow.Mainnet.Chain(),
		ServiceAccountEnabled: true,

		RuntimeParams:         DefaultRuntimeParams(),
		ProgramLoggerParams:   DefaultProgramLoggerParams(),
		EventEmitterParams:    DefaultEventEmitterParams(),
		BlockInfoParams:       DefaultBlockInfoParams(),
		TransactionInfoParams: DefaultTransactionInfoParams(),
		ContractUpdaterParams: DefaultContractUpdaterParams(),
	}
}

func (env *EnvironmentParams) SetScriptInfoParams(info *ScriptInfoParams) {
	env.ScriptInfoParams = *info
}
