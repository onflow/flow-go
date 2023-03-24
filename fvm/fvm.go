package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
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
		txnState storage.Transaction,
	) ProcedureExecutor

	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
	ShouldDisableMemoryAndInteractionLimits(ctx Context) bool

	Type() ProcedureType

	// For transactions, the execution time is TxIndex.  For scripts, the
	// execution time is EndOfBlockExecutionTime.
	ExecutionTime() logical.Time

	// TODO(patrick): deprecated this.
	SetOutput(output ProcedureOutput)
}

// VM runs procedures
type VM interface {
	RunV2(
		Context,
		Procedure,
		state.StorageSnapshot,
	) (
		*state.ExecutionSnapshot,
		ProcedureOutput,
		error,
	)

	Run(Context, Procedure, state.View) error
	GetAccount(Context, flow.Address, state.StorageSnapshot) (*flow.Account, error)
}

var _ VM = (*VirtualMachine)(nil)

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
}

func NewVirtualMachine() *VirtualMachine {
	return &VirtualMachine{}
}

// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) RunV2(
	ctx Context,
	proc Procedure,
	storageSnapshot state.StorageSnapshot,
) (
	*state.ExecutionSnapshot,
	ProcedureOutput,
	error,
) {
	derivedBlockData := ctx.DerivedBlockData
	if derivedBlockData == nil {
		derivedBlockData = derived.NewEmptyDerivedBlockDataWithTransactionOffset(
			uint32(proc.ExecutionTime()))
	}

	var derivedTxnData derived.DerivedTransactionCommitter
	var err error
	switch proc.Type() {
	case ScriptProcedureType:
		derivedTxnData, err = derivedBlockData.NewSnapshotReadDerivedTransactionData(
			proc.ExecutionTime(),
			proc.ExecutionTime())
	case TransactionProcedureType, BootstrapProcedureType:
		derivedTxnData, err = derivedBlockData.NewDerivedTransactionData(
			proc.ExecutionTime(),
			proc.ExecutionTime())
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

	// TODO(patrick): initialize view inside TransactionState
	nestedTxn := state.NewTransactionState(
		delta.NewDeltaView(storageSnapshot),
		state.DefaultParameters().
			WithMeterParameters(getBasicMeterParameters(ctx, proc)).
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize))

	txnState := &storage.SerialTransaction{
		NestedTransaction:           nestedTxn,
		DerivedTransactionCommitter: derivedTxnData,
	}

	executor := proc.NewExecutor(ctx, txnState)
	err = Run(executor)
	if err != nil {
		return nil, ProcedureOutput{}, err
	}

	// Note: it is safe to skip committing derived data for non-normal
	// transactions (i.e., bootstrap and script) since these do not invalidate
	// derived data entries.
	if proc.Type() == TransactionProcedureType {
		// NOTE: It is not safe to ignore derivedTxnData' commit error for
		// transactions that trigger derived data invalidation.
		err = derivedTxnData.Commit()
		if err != nil {
			return nil, ProcedureOutput{}, err
		}
	}

	executionSnapshot, err := txnState.FinalizeMainTransaction()
	if err != nil {
		return nil, ProcedureOutput{}, err
	}

	return executionSnapshot, executor.Output(), nil
}

func (vm *VirtualMachine) Run(
	ctx Context,
	proc Procedure,
	v state.View,
) error {
	executionSnapshot, output, err := vm.RunV2(
		ctx,
		proc,
		state.NewPeekerStorageSnapshot(v))
	if err != nil {
		return err
	}

	err = v.Merge(executionSnapshot)
	if err != nil {
		return err
	}

	proc.SetOutput(output)
	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(
	ctx Context,
	address flow.Address,
	storageSnapshot state.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	nestedTxn := state.NewTransactionState(
		// TODO(patrick): initialize view inside TransactionState
		delta.NewDeltaView(storageSnapshot),
		state.DefaultParameters().
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMeterParameters(
				meter.DefaultParameters().
					WithStorageInteractionLimit(ctx.MaxStateInteractionSize)))

	derivedBlockData := ctx.DerivedBlockData
	if derivedBlockData == nil {
		derivedBlockData = derived.NewEmptyDerivedBlockData()
	}

	derivedTxnData, err := derivedBlockData.NewSnapshotReadDerivedTransactionData(
		logical.EndOfBlockExecutionTime,
		logical.EndOfBlockExecutionTime)
	if err != nil {
		return nil, fmt.Errorf(
			"error creating derived transaction data for GetAccount: %w",
			err)
	}

	txnState := &storage.SerialTransaction{
		NestedTransaction:           nestedTxn,
		DerivedTransactionCommitter: derivedTxnData,
	}

	env := environment.NewScriptEnv(
		context.Background(),
		ctx.TracerSpan,
		ctx.EnvironmentParams,
		txnState)
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
