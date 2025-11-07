package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

type ProcedureType string

const (
	BootstrapProcedureType   = ProcedureType("bootstrap")
	TransactionProcedureType = ProcedureType("transaction")
	ScriptProcedureType      = ProcedureType("script")
)

type ProcedureOutput struct {
	// Output by both transaction and script.
	Logs                   []string
	Events                 flow.EventsList
	ServiceEvents          flow.EventsList
	ConvertedServiceEvents flow.ServiceEventList
	ComputationUsed        uint64
	ComputationIntensities meter.MeteredComputationIntensities
	MemoryEstimate         uint64
	Err                    errors.CodedError

	// Output only by script.
	Value cadence.Value
}

func (output *ProcedureOutput) PopulateEnvironmentValues(
	env environment.Environment,
) error {
	output.Logs = env.Logs()

	computationUsed, err := env.ComputationUsed()
	if err != nil {
		return fmt.Errorf("error getting computation used: %w", err)
	}
	output.ComputationUsed = computationUsed

	memoryUsed, err := env.MemoryUsed()
	if err != nil {
		return fmt.Errorf("error getting memory used: %w", err)
	}
	output.MemoryEstimate = memoryUsed

	output.ComputationIntensities = env.ComputationIntensities()

	// if tx failed this will only contain fee deduction events
	output.Events = env.Events()
	output.ServiceEvents = env.ServiceEvents()
	output.ConvertedServiceEvents = env.ConvertedServiceEvents()

	return nil
}

type ProcedureExecutor interface {
	Preprocess() error
	Execute() error
	Cleanup()

	Output() ProcedureOutput
}

func Run(executor ProcedureExecutor) error {
	defer executor.Cleanup()
	err := executor.Preprocess()
	if err != nil {
		return err
	}
	return executor.Execute()
}

// RunWithLogger runs an executor with logging support
func RunWithLogger(executor ProcedureExecutor, logger zerolog.Logger) error {
	defer executor.Cleanup()
	
	logger.Info().Msg("executor.Run: starting Preprocess")
	err := executor.Preprocess()
	logger.Info().
		Err(err).
		Msg("executor.Run: Preprocess completed")
	if err != nil {
		return err
	}
	
	logger.Info().Msg("executor.Run: starting Execute")
	err = executor.Execute()
	logger.Info().
		Err(err).
		Msg("executor.Run: Execute completed")
	return err
}

// A Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	NewExecutor(
		ctx Context,
		txnState storage.TransactionPreparer,
	) ProcedureExecutor

	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
	ShouldDisableMemoryAndInteractionLimits(ctx Context) bool

	Type() ProcedureType

	// For transactions, the execution time is TxIndex.  For scripts, the
	// execution time is EndOfBlockExecutionTime.
	ExecutionTime() logical.Time
}

// VM runs procedures
type VM interface {
	NewExecutor(
		Context,
		Procedure,
		storage.TransactionPreparer,
	) ProcedureExecutor

	Run(
		Context,
		Procedure,
		snapshot.StorageSnapshot,
	) (
		*snapshot.ExecutionSnapshot,
		ProcedureOutput,
		error,
	)
}

var _ VM = (*VirtualMachine)(nil)

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
}

func NewVirtualMachine() *VirtualMachine {
	return &VirtualMachine{}
}

func (vm *VirtualMachine) NewExecutor(
	ctx Context,
	proc Procedure,
	txn storage.TransactionPreparer,
) ProcedureExecutor {
	return proc.NewExecutor(ctx, txn)
}

// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) Run(
	ctx Context,
	proc Procedure,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	ProcedureOutput,
	error,
) {
	logger := ctx.Logger
	if logger.GetLevel() == zerolog.Disabled {
		logger = zerolog.Nop()
	}

	logger.Info().
		Str("procedure_type", string(proc.Type())).
		Uint64("execution_time", uint64(proc.ExecutionTime())).
		Msg("vm.Run: starting")

	blockDatabase := storage.NewBlockDatabase(
		storageSnapshot,
		proc.ExecutionTime(),
		ctx.DerivedBlockData)

	logger.Info().Msg("vm.Run: created blockDatabase")

	stateParameters := ProcedureStateParameters(ctx, proc)

	var storageTxn storage.Transaction
	var err error
	switch proc.Type() {
	case ScriptProcedureType:
		if ctx.AllowProgramCacheWritesInScripts {
			// if configured, allow scripts to update the programs cache
			logger.Info().Msg("vm.Run: creating caching snapshot read transaction")
			storageTxn, err = blockDatabase.NewCachingSnapshotReadTransaction(stateParameters)
		} else {
			logger.Info().Msg("vm.Run: creating snapshot read transaction")
			storageTxn = blockDatabase.NewSnapshotReadTransaction(stateParameters)
		}
	case TransactionProcedureType, BootstrapProcedureType:
		logger.Info().Msg("vm.Run: creating new transaction")
		storageTxn, err = blockDatabase.NewTransaction(
			proc.ExecutionTime(),
			stateParameters)
	default:
		return nil, ProcedureOutput{}, fmt.Errorf(
			"invalid proc type: %v",
			proc.Type())
	}

	if err != nil {
		logger.Error().Err(err).Msg("vm.Run: error creating transaction")
		return nil, ProcedureOutput{}, fmt.Errorf(
			"error creating derived transaction data: %w",
			err)
	}

	logger.Info().Msg("vm.Run: created storage transaction, creating executor")

	executor := proc.NewExecutor(ctx, storageTxn)

	logger.Info().Msg("vm.Run: created executor, calling executor.Run (Preprocess + Execute)")

	err = RunWithLogger(executor, logger)

	logger.Info().
		Err(err).
		Msg("vm.Run: executor.Run completed")

	if err != nil {
		return nil, ProcedureOutput{}, err
	}

	logger.Info().Msg("vm.Run: finalizing storage transaction")

	err = storageTxn.Finalize()
	if err != nil {
		logger.Error().Err(err).Msg("vm.Run: error finalizing transaction")
		return nil, ProcedureOutput{}, err
	}

	logger.Info().Msg("vm.Run: committing storage transaction")

	executionSnapshot, err := storageTxn.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("vm.Run: error committing transaction")
		return nil, ProcedureOutput{}, err
	}

	logger.Info().Msg("vm.Run: completed successfully")

	return executionSnapshot, executor.Output(), nil
}

// GetAccount returns an account by address or an error if none exists.
func GetAccount(
	ctx Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	env, _ := getScriptEnvironment(ctx, storageSnapshot)

	account, err := env.GetAccount(address)
	if err != nil {
		if errors.IsLedgerFailure(err) {
			return nil, fmt.Errorf(
				"cannot get account, this error usually happens if the "+
					"reference block for this query is not set to a recent "+
					"block: %w",
				err)
		}
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	return account, nil
}

// GetAccountBalance returns an account balance by address or an error if none exists.
func GetAccountBalance(
	ctx Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	uint64,
	error,
) {
	env, _ := getScriptEnvironment(ctx, storageSnapshot)

	accountBalance, err := env.GetAccountBalance(common.MustBytesToAddress(address.Bytes()))

	if err != nil {
		return 0, fmt.Errorf("cannot get account balance: %w", err)
	}
	return accountBalance, nil
}

// GetAccountAvailableBalance returns an account available balance by address or an error if none exists.
func GetAccountAvailableBalance(
	ctx Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	uint64,
	error,
) {
	env, _ := getScriptEnvironment(ctx, storageSnapshot)

	accountBalance, err := env.GetAccountAvailableBalance(common.MustBytesToAddress(address.Bytes()))

	if err != nil {
		return 0, fmt.Errorf("cannot get account balance: %w", err)
	}
	return accountBalance, nil
}

// GetAccountKeys returns an account keys by address or an error if none exists.
func GetAccountKeys(
	ctx Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	[]flow.AccountPublicKey,
	error,
) {
	_, accountInfo := getScriptEnvironment(ctx, storageSnapshot)
	accountKeys, err := accountInfo.GetAccountKeys(address)

	if err != nil {
		return nil, fmt.Errorf("cannot get account keys: %w", err)
	}
	return accountKeys, nil
}

// GetAccountKey returns an account key by address and index or an error if none exists.
func GetAccountKey(
	ctx Context,
	address flow.Address,
	keyIndex uint32,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*flow.AccountPublicKey,
	error,
) {
	_, accountInfo := getScriptEnvironment(ctx, storageSnapshot)
	accountKey, err := accountInfo.GetAccountKeyByIndex(address, keyIndex)

	if err != nil {
		return nil, fmt.Errorf("cannot get account keys: %w", err)
	}

	return accountKey, nil
}

// Helper function to initialize common components.
func getScriptEnvironment(
	ctx Context,
	storageSnapshot snapshot.StorageSnapshot,
) (environment.Environment, environment.AccountInfo) {
	blockDatabase := storage.NewBlockDatabase(
		storageSnapshot,
		0,
		ctx.DerivedBlockData)

	storageTxn := blockDatabase.NewSnapshotReadTransaction(
		state.DefaultParameters().
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMeterParameters(
				meter.DefaultParameters().
					WithStorageInteractionLimit(ctx.MaxStateInteractionSize)))

	env := environment.NewScriptEnv(
		context.Background(),
		ctx.TracerSpan,
		ctx.EnvironmentParams,
		storageTxn)

	return env, env.AccountInfo
}
