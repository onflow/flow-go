package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/blueprints"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter/weighted"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error
	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
}

func NewInterpreterRuntime(options ...runtime.Option) runtime.Runtime {

	defaultOptions := []runtime.Option{
		runtime.WithContractUpdateValidationEnabled(true),
	}

	return runtime.NewInterpreterRuntime(
		append(defaultOptions, options...)...,
	)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	Runtime runtime.Runtime
}

// NewVirtualMachine creates a new virtual machine instance with the provided runtime.
func NewVirtualMachine(rt runtime.Runtime) *VirtualMachine {
	return &VirtualMachine{
		Runtime: rt,
	}
}

// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) Run(ctx Context, proc Procedure, v state.View, programs *programs.Programs) (err error) {
	st := state.NewState(v,
		state.WithMeter(weighted.NewMeter(
			uint(proc.ComputationLimit(ctx)),
			uint(proc.MemoryLimit(ctx)))),
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))
	sth := state.NewStateHolder(st)

	err = proc.Run(vm, ctx, sth, programs)
	if err != nil {
		return err
	}

	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(ctx Context, address flow.Address, v state.View, programs *programs.Programs) (*flow.Account, error) {
	st := state.NewState(v,
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))

	sth := state.NewStateHolder(st)
	account, err := getAccount(vm, ctx, sth, programs, address)
	if err != nil {
		if errors.IsALedgerFailure(err) {
			return nil, fmt.Errorf("cannot get account, this error usually happens if the reference block for this query is not set to a recent block: %w", err)
		}
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	return account, nil
}

// invokeMetaTransaction invokes a meta transaction inside the context of an outer transaction.
//
// Errors that occur in a meta transaction are propagated as a single error that can be
// captured by the Cadence runtime and eventually disambiguated by the parent context.
func (vm *VirtualMachine) invokeMetaTransaction(parentCtx Context, tx *TransactionProcedure, sth *state.StateHolder, programs *programs.Programs) (errors.Error, error) {
	invoker := NewTransactionInvoker(zerolog.Nop())

	// do not deduct fees or check storage in meta transactions
	ctx := NewContextFromParent(parentCtx,
		WithAccountStorageLimit(false),
		WithTransactionFeesEnabled(false),
	)

	err := invoker.Process(vm, &ctx, tx, sth, programs)
	txErr, fatalErr := errors.SplitErrorTypes(err)
	return txErr, fatalErr
}

// getExecutionWeights reads stored execution effort weights from the service account
func getExecutionEffortWeights(
	env Environment,
	accounts state.Accounts,
) (
	computationWeights weighted.ExecutionEffortWeights,
	err error,
) {
	// the weights are stored in the service account
	serviceAddress := env.Context().Chain.ServiceAddress()

	service := runtime.Address(serviceAddress)
	// Check that the service account exists
	ok, err := accounts.Exists(serviceAddress)

	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}
	if !ok {
		// if the service account does not exist, return an FVM error
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionFeesExecutionEffortWeightsPathDomain,
			blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier)
	}

	value, err := env.VM().Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.TransactionFeesExecutionEffortWeightsPathDomain,
			Identifier: blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier,
		},
		runtime.Context{Interface: env},
	)
	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}

	computationWeightsRaw, ok := utils.CadenceValueToWeights(value)
	if !ok {
		// this is a non-fatal error. It is expected if the weights are not set up on the network yet.
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionFeesExecutionEffortWeightsPathDomain,
			blueprints.TransactionFeesExecutionEffortWeightsPathIdentifier)
	}

	// Merge the default weights with the weights from the state.
	// This allows for weights that are not set in the state, to be set by default.
	// In case the network is stuck because of a transaction using an FVM feature that has 0 weight
	// (or is not metered at all), the defaults can be changed and the network restarted
	// instead of trying to change the weights with a transaction.
	computationWeights = make(weighted.ExecutionEffortWeights)
	for k, v := range weighted.DefaultComputationWeights {
		computationWeights[k] = v
	}
	for k, v := range computationWeightsRaw {
		computationWeights[common.ComputationKind(k)] = v
	}

	return computationWeights, nil
}

// getExecutionMemoryWeights reads stored execution memory weights from the service account
func getExecutionMemoryWeights(
	env Environment,
	accounts state.Accounts,
) (
	memoryWeights weighted.ExecutionMemoryWeights,
	err error,
) {
	// the weights are stored in the service account
	serviceAddress := env.Context().Chain.ServiceAddress()

	service := runtime.Address(serviceAddress)
	// Check that the service account exists
	ok, err := accounts.Exists(serviceAddress)

	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}
	if !ok {
		// if the service account does not exist, return an FVM error
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionFeesExecutionMemoryWeightsPathDomain,
			blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier)
	}

	value, err := env.VM().Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.TransactionFeesExecutionMemoryWeightsPathDomain,
			Identifier: blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier,
		},
		runtime.Context{Interface: env},
	)
	if err != nil {
		// this might be fatal, return as is
		return nil, err
	}

	memoryWeightsRaw, ok := utils.CadenceValueToWeights(value)
	if !ok {
		// this is a non-fatal error. It is expected if the weights are not set up on the network yet.
		return nil, errors.NewCouldNotGetExecutionParameterFromStateError(
			service.Hex(),
			blueprints.TransactionFeesExecutionMemoryWeightsPathDomain,
			blueprints.TransactionFeesExecutionMemoryWeightsPathIdentifier)
	}

	// Merge the default weights with the weights from the state.
	// This allows for weights that are not set in the state, to be set by default.
	// In case the network is stuck because of a transaction using an FVM feature that has 0 weight
	// (or is not metered at all), the defaults can be changed and the network restarted
	// instead of trying to change the weights with a transaction.
	memoryWeights = make(weighted.ExecutionMemoryWeights)
	for k, v := range weighted.DefaultMemoryWeights {
		memoryWeights[k] = v
	}
	for k, v := range memoryWeightsRaw {
		memoryWeights[common.MemoryKind(k)] = v
	}

	return memoryWeights, nil
}
