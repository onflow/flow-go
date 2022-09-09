package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// TODO(patrick): rm after emulator is updated
type Environment = environment.Environment

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
	environment.AccountFreezer
	*environment.ValueStore
	*environment.ContractReader
	*environment.AccountKeyReader
	*environment.SystemContracts

	handler.ContractUpdater

	// TODO(patrick): rm
	ctx Context

	sth         *state.StateHolder
	vm          *VirtualMachine
	programs    *handler.ProgramsHandler
	accounts    environment.Accounts
	accountKeys *handler.AccountKeyHandler

	// TODO(patrick): rm once fully refactored
	fullEnv environment.Environment
}

func newCommonEnv(
	ctx Context,
	vm *VirtualMachine,
	stateTransaction *state.StateHolder,
	programs handler.TransactionPrograms,
	tracer *environment.Tracer,
	meter environment.Meter,
) commonEnv {
	accounts := environment.NewAccounts(stateTransaction)
	programsHandler := handler.NewProgramsHandler(programs, stateTransaction)

	return commonEnv{
		Tracer: tracer,
		Meter:  meter,
		ProgramLogger: environment.NewProgramLogger(
			tracer,
			ctx.Logger,
			ctx.Metrics,
			ctx.CadenceLoggingEnabled,
		),
		UUIDGenerator: environment.NewUUIDGenerator(
			tracer,
			meter,
			stateTransaction),
		UnsafeRandomGenerator: environment.NewUnsafeRandomGenerator(
			tracer,
			ctx.BlockHeader,
		),
		CryptoLibrary: environment.NewCryptoLibrary(tracer, meter),
		BlockInfo: environment.NewBlockInfo(
			tracer,
			meter,
			ctx.BlockHeader,
			ctx.Blocks,
		),
		ValueStore: environment.NewValueStore(
			tracer,
			meter,
			accounts,
		),
		ContractReader: environment.NewContractReader(
			tracer,
			meter,
			accounts,
		),
		AccountKeyReader: environment.NewAccountKeyReader(
			tracer,
			meter,
			accounts,
		),
		SystemContracts: environment.NewSystemContracts(),
		ctx:             ctx,
		sth:             stateTransaction,
		vm:              vm,
		programs:        programsHandler,
		accounts:        accounts,
	}
}

func (env *commonEnv) Chain() flow.Chain {
	return env.ctx.Chain
}

func (env *commonEnv) LimitAccountStorage() bool {
	return env.ctx.LimitAccountStorage
}

func (env *commonEnv) BorrowCadenceRuntime() *runtime.ReusableCadenceRuntime {
	return env.ctx.ReusableCadenceRuntimePool.Borrow(env.fullEnv)
}

func (env *commonEnv) ReturnCadenceRuntime(
	reusable *runtime.ReusableCadenceRuntime,
) {
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
	keys, err := env.ContractUpdater.Commit()
	return programs.ModifiedSets{
		ContractUpdateKeys: keys,
		FrozenAccounts:     env.FrozenAccounts(),
	}, err
}

func (commonEnv) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}
