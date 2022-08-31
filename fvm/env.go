package fvm

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"go.opentelemetry.io/otel/attribute"
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
	// TODO(patrick): stop exposing Context()
	Context() *Context

	VM() *VirtualMachine
	runtime.Interface

	StartSpanFromRoot(name trace.SpanName) otelTrace.Span
	StartExtensiveTracingSpanFromRoot(name trace.SpanName) otelTrace.Span

	BorrowCadenceRuntime() *ReusableCadenceRuntime
	ReturnCadenceRuntime(*ReusableCadenceRuntime)

	SetAccountFrozen(address common.Address, frozen bool) error
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

	// TODO(patrick): rm
	ctx Context

	AccountInterface

	sth         *state.StateHolder
	vm          *VirtualMachine
	programs    *handler.ProgramsHandler
	accounts    state.Accounts
	accountKeys *handler.AccountKeyHandler
	contracts   *handler.ContractHandler

	frozenAccounts []common.Address

	// TODO(patrick): rm once fully refactored
	fullEnv Environment
}

func (env *commonEnv) Context() *Context {
	return &env.ctx
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

func (env *commonEnv) GetValue(owner, key []byte) ([]byte, error) {
	var valueByteSize int
	span := env.StartSpanFromRoot(trace.FVMEnvGetValue)
	defer func() {
		if !trace.IsSampled(span) {
			span.SetAttributes(
				attribute.String("owner", hex.EncodeToString(owner)),
				attribute.String("key", string(key)),
				attribute.Int("valueByteSize", valueByteSize),
			)
		}
		span.End()
	}()

	v, err := env.accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	valueByteSize = len(v)

	err = env.MeterComputation(environment.ComputationKindGetValue, uint(valueByteSize))
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	return v, nil
}

// TODO disable SetValue for scripts, right now the view changes are discarded
func (env *commonEnv) SetValue(owner, key, value []byte) error {
	span := env.StartSpanFromRoot(trace.FVMEnvSetValue)
	if !trace.IsSampled(span) {
		span.SetAttributes(
			attribute.String("owner", hex.EncodeToString(owner)),
			attribute.String("key", string(key)),
		)
	}
	defer span.End()

	err := env.MeterComputation(environment.ComputationKindSetValue, uint(len(value)))
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}

	err = env.accounts.SetValue(
		flow.BytesToAddress(owner),
		string(key),
		value,
	)
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}
	return nil
}

func (env *commonEnv) ValueExists(owner, key []byte) (exists bool, err error) {
	defer env.StartSpanFromRoot(trace.FVMEnvValueExists).End()

	err = env.MeterComputation(environment.ComputationKindValueExists, 1)
	if err != nil {
		return false, fmt.Errorf("check value existence failed: %w", err)
	}

	v, err := env.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("check value existence failed: %w", err)
	}

	return len(v) > 0, nil
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

	result, invokeErr := InvokeAccountStorageCapacityContract(
		env.fullEnv,
		address)
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

	result, invokeErr := InvokeAccountBalanceContract(env.fullEnv, address)
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

	result, invokeErr := InvokeAccountAvailableBalanceContract(
		env.fullEnv,
		address)

	if invokeErr != nil {
		return 0, invokeErr
	}
	return result.ToGoValue().(uint64), nil
}

func (env *commonEnv) GetAccountContractNames(address runtime.Address) ([]string, error) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetAccountContractNames).End()

	err := env.MeterComputation(environment.ComputationKindGetAccountContractNames, 1)
	if err != nil {
		return nil, fmt.Errorf("get account contract names failed: %w", err)
	}

	a := flow.Address(address)

	freezeError := env.accounts.CheckAccountNotFrozen(a)
	if freezeError != nil {
		return nil, fmt.Errorf("get account contract names failed: %w", freezeError)
	}

	return env.accounts.GetContractNames(a)
}

func (env *commonEnv) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	defer env.StartExtensiveTracingSpanFromRoot(trace.FVMEnvResolveLocation).End()

	err := env.MeterComputation(environment.ComputationKindResolveLocation, 1)
	if err != nil {
		return nil, fmt.Errorf("resolve location failed: %w", err)
	}

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location
	// (`import Crypto`), then return a single resolved location which declares
	// all identifiers.
	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address
	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)

		err := env.accounts.CheckAccountNotFrozen(address)
		if err != nil {
			return nil, fmt.Errorf("resolving location's account frozen check failed: %w", err)
		}

		contractNames, err := env.contracts.GetContractNames(
			addressLocation.Address)
		if err != nil {
			return nil, fmt.Errorf("resolving location failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]ast.Identifier, len(contractNames))

		for i := range identifiers {
			identifiers[i] = runtime.Identifier{
				Identifier: contractNames[i],
			}
		}
	}

	// return one resolved location per identifier.
	// each resolved location is an address contract location
	resolvedLocations := make([]runtime.ResolvedLocation, len(identifiers))
	for i := range resolvedLocations {
		identifier := identifiers[i]
		resolvedLocations[i] = runtime.ResolvedLocation{
			Location: common.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations, nil
}

func (env *commonEnv) GetCode(location runtime.Location) ([]byte, error) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetCode).End()

	err := env.MeterComputation(environment.ComputationKindGetCode, 1)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, errors.NewInvalidLocationErrorf(location, "expecting an AddressLocation, but other location types are passed")
	}

	address := flow.Address(contractLocation.Address)

	err = env.accounts.CheckAccountNotFrozen(address)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	add, err := env.contracts.GetContract(contractLocation.Address, contractLocation.Name)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	return add, nil
}

func (env *commonEnv) GetAccountContractCode(address runtime.Address, name string) (code []byte, err error) {
	defer env.StartSpanFromRoot(trace.FVMEnvGetAccountContractCode).End()

	err = env.MeterComputation(environment.ComputationKindGetAccountContractCode, 1)
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	code, err = env.GetCode(common.AddressLocation{
		Address: address,
		Name:    name,
	})
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	return code, nil
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
