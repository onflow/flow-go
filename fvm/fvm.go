package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/environment"
	errors "github.com/onflow/flow-go/fvm/errors"
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

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
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

	GetAccount(Context, flow.Address, snapshot.StorageSnapshot) (*flow.Account, error)
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
	blockDatabase := storage.NewBlockDatabase(
		storageSnapshot,
		proc.ExecutionTime(),
		ctx.DerivedBlockData)

	stateParameters := ProcedureStateParameters(ctx, proc)

	var storageTxn storage.Transaction
	var err error
	switch proc.Type() {
	case ScriptProcedureType:
		storageTxn = blockDatabase.NewSnapshotReadTransaction(stateParameters)
	case TransactionProcedureType, BootstrapProcedureType:
		storageTxn, err = blockDatabase.NewTransaction(
			proc.ExecutionTime(),
			stateParameters)
	default:
		return nil, ProcedureOutput{}, fmt.Errorf(
			"invalid proc type: %v",
			proc.Type())
	}

	if err != nil {
		return nil, ProcedureOutput{}, fmt.Errorf(
			"error creating derived transaction data: %w",
			err)
	}

	executor := proc.NewExecutor(ctx, storageTxn)
	err = Run(executor)
	if err != nil {
		return nil, ProcedureOutput{}, err
	}

	err = storageTxn.Finalize()
	if err != nil {
		return nil, ProcedureOutput{}, err
	}

	executionSnapshot, err := storageTxn.Commit()
	if err != nil {
		return nil, ProcedureOutput{}, err
	}

	return executionSnapshot, executor.Output(), nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(
	ctx Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
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
