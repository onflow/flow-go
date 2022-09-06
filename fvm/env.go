package fvm

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// Environment accepts a context and a virtual machine instance and provides
// cadence runtime interface methods to the runtime.
type Environment interface {
	VM() *VirtualMachine
	runtime.Interface

	Chain() flow.Chain

	LimitAccountStorage() bool

	StartSpanFromRoot(name trace.SpanName) otelTrace.Span
	StartExtensiveTracingSpanFromRoot(name trace.SpanName) otelTrace.Span

	Logger() *zerolog.Logger

	BorrowCadenceRuntime() *ReusableCadenceRuntime
	ReturnCadenceRuntime(*ReusableCadenceRuntime)

	SetAccountFrozen(address common.Address, frozen bool) error

	AccountsStorageCapacity(addresses []common.Address) (cadence.Value, error)
}

// TODO(patrick): refactor this into an object
// Set of account api that do not have shared implementation.
type AccountInterface interface {
	CreateAccount(address runtime.Address) (runtime.Address, error)

	AddAccountKey(
		address runtime.Address,
		publicKey *runtime.PublicKey,
		hashAlgo runtime.HashAlgorithm,
		weight int,
	) (
		*runtime.AccountKey,
		error,
	)

	GetAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error)

	RevokeAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error)

	AddEncodedAccountKey(address runtime.Address, publicKey []byte) error
}

// Parts of the environment that are common to all transaction and script
// executions.
type commonEnv struct {
	*environment.Tracer
	environment.Meter
	*environment.ProgramLogger
	*environment.UUIDGenerator
	*environment.UnsafeRandomGenerator
	*environment.CryptoLibrary
	*environment.BlockInfo
	environment.TransactionInfo
	environment.EventEmitter
	*environment.ValueStore
	*environment.ContractReader

	*SystemContracts

	// TODO(patrick): rm
	ctx Context

	AccountInterface

	sth         *state.StateHolder
	vm          *VirtualMachine
	programs    *handler.ProgramsHandler
	accounts    environment.Accounts
	accountKeys *handler.AccountKeyHandler
	contracts   *handler.ContractHandler

	frozenAccounts []common.Address

	// TODO(patrick): rm once fully refactored
	fullEnv Environment
}

func (env *commonEnv) Chain() flow.Chain {
	return env.ctx.Chain
}

func (env *commonEnv) LimitAccountStorage() bool {
	return env.ctx.LimitAccountStorage
}

func (env *commonEnv) VM() *VirtualMachine {
	return env.vm
}

func (env *commonEnv) BorrowCadenceRuntime() *ReusableCadenceRuntime {
	return env.ctx.ReusableCadenceRuntimePool.Borrow(env.fullEnv)
}

func (env *commonEnv) ReturnCadenceRuntime(reusable *ReusableCadenceRuntime) {
	env.ctx.ReusableCadenceRuntimePool.Return(reusable)
}

func (env *commonEnv) GetStorageUsed(address common.Address) (value uint64, err error) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetStorageUsed).End()

	err = env.MeterComputation(environment.ComputationKindGetStorageUsed, 1)
	if err != nil {
		return value, fmt.Errorf("get storage used failed: %w", err)
	}

	value, err = env.accounts.GetStorageUsed(flow.Address(address))
	if err != nil {
		return value, fmt.Errorf("get storage used failed: %w", err)
	}

	return value, nil
}

// storageMBUFixToBytesUInt converts the return type of storage capacity which is a UFix64 with the unit of megabytes to
// UInt with the unit of bytes
func storageMBUFixToBytesUInt(result cadence.Value) uint64 {
	// Divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return result.ToGoValue().(uint64) / 100
}

func (env *commonEnv) GetStorageCapacity(
	address common.Address,
) (
	value uint64,
	err error,
) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetStorageCapacity).End()

	err = env.MeterComputation(environment.ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return 0, fmt.Errorf("get storage capacity failed: %w", err)
	}

	result, invokeErr := env.AccountStorageCapacity(address)
	if invokeErr != nil {
		return 0, invokeErr
	}

	// Return type is actually a UFix64 with the unit of megabytes so some
	// conversion is necessary divide the unsigned int by (1e8 (the scale of
	// Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return storageMBUFixToBytesUInt(result), nil
}

func (env *commonEnv) GetAccountBalance(
	address common.Address,
) (
	value uint64,
	err error,
) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetAccountBalance).End()

	err = env.MeterComputation(environment.ComputationKindGetAccountBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account balance failed: %w", err)
	}

	result, invokeErr := env.AccountBalance(address)
	if invokeErr != nil {
		return 0, invokeErr
	}
	return result.ToGoValue().(uint64), nil
}

func (env *commonEnv) GetAccountAvailableBalance(
	address common.Address,
) (
	value uint64,
	err error,
) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetAccountBalance).End()

	err = env.MeterComputation(environment.ComputationKindGetAccountAvailableBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account available balance failed: %w", err)
	}

	result, invokeErr := env.AccountAvailableBalance(address)
	if invokeErr != nil {
		return 0, invokeErr
	}
	return result.ToGoValue().(uint64), nil
}

func (env *commonEnv) GetProgram(location common.Location) (*interpreter.Program, error) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetProgram).End()

	err := env.MeterComputation(environment.ComputationKindGetProgram, 1)
	if err != nil {
		return nil, fmt.Errorf("get program failed: %w", err)
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		address := flow.Address(addressLocation.Address)

		freezeError := env.accounts.CheckAccountNotFrozen(address)
		if freezeError != nil {
			return nil, fmt.Errorf("get program failed: %w", freezeError)
		}
	}

	program, has := env.programs.Get(location)
	if has {
		return program, nil
	}

	return nil, nil
}

func (env *commonEnv) SetProgram(location common.Location, program *interpreter.Program) error {
	defer env.StartSpanFromRoot(trace.FVMEnvSetProgram).End()

	err := env.MeterComputation(environment.ComputationKindSetProgram, 1)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}

	err = env.programs.Set(location, program)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}
	return nil
}

func (env *commonEnv) DecodeArgument(b []byte, _ cadence.Type) (cadence.Value, error) {
	defer env.StartExtensiveTracingSpanFromRoot(trace.FVMEnvDecodeArgument).End()

	v, err := jsoncdc.Decode(env, b)
	if err != nil {
		err = errors.NewInvalidArgumentErrorf("argument is not json decodable: %w", err)
		return nil, fmt.Errorf("decodeing argument failed: %w", err)
	}

	return v, err
}

// Commit commits changes and return a list of updated keys
func (env *commonEnv) Commit() (programs.ModifiedSets, error) {
	keys, err := env.contracts.Commit()
	return programs.ModifiedSets{
		ContractUpdateKeys: keys,
		FrozenAccounts:     env.frozenAccounts,
	}, err
}

func (commonEnv) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (env *commonEnv) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	err := env.MeterComputation(environment.ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("allocate storage index failed: %w", err)
	}

	v, err := env.accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}

// GetAccountKey retrieves a public key by index from an existing account.
//
// This function returns a nil key with no errors, if a key doesn't exist at
// the given index. An error is returned if the specified account does not
// exist, the provided index is not valid, or if the key retrieval fails.
func (env *commonEnv) GetAccountKey(
	address runtime.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetAccountKey).End()

	err := env.MeterComputation(environment.ComputationKindGetAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}

	accKey, err := env.accountKeys.GetAccountKey(address, keyIndex)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}
	return accKey, err
}
