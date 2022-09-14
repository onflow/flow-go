package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
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
	*environment.Runtime

	*environment.UUIDGenerator

	*environment.UnsafeRandomGenerator
	*environment.CryptoLibrary

	*environment.BlockInfo
	*environment.AccountInfo
	environment.TransactionInfo

	environment.EventEmitter

	environment.AccountCreator
	environment.AccountFreezer

	*environment.ValueStore
	*environment.ContractReader
	*environment.AccountKeyReader
	*environment.SystemContracts

	handler.ContractUpdater
	handler.AccountKeyUpdater

	// TODO(patrick): rm
	ctx Context

	sth      *state.StateHolder
	programs *handler.ProgramsHandler
	accounts environment.Accounts

	// TODO(patrick): rm once fully refactored
	fullEnv environment.Environment
}

func newCommonEnv(
	ctx Context,
	stateTransaction *state.StateHolder,
	programs handler.TransactionPrograms,
	tracer *environment.Tracer,
	meter environment.Meter,
) commonEnv {
	accounts := environment.NewAccounts(stateTransaction)
	programsHandler := handler.NewProgramsHandler(programs, stateTransaction)
	logger := environment.NewProgramLogger(
		tracer,
		ctx.Logger,
		ctx.Metrics,
		ctx.CadenceLoggingEnabled,
	)
	runtime := environment.NewRuntime(ctx.ReusableCadenceRuntimePool)
	systemContracts := environment.NewSystemContracts(
		ctx.Chain,
		tracer,
		logger,
		runtime)

	return commonEnv{
		Tracer:        tracer,
		Meter:         meter,
		ProgramLogger: logger,
		Runtime:       runtime,
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
		AccountInfo: environment.NewAccountInfo(
			tracer,
			meter,
			accounts,
			systemContracts,
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
		SystemContracts: systemContracts,
		ctx:             ctx,
		sth:             stateTransaction,
		programs:        programsHandler,
		accounts:        accounts,
	}
}

func (env *commonEnv) Context() Context {
	return env.ctx
}

func (env *commonEnv) Chain() flow.Chain {
	return env.ctx.Chain
}

func (env *commonEnv) LimitAccountStorage() bool {
	return env.ctx.LimitAccountStorage
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

func (env *commonEnv) DecodeArgument(b []byte, _ cadence.Type) (v cadence.Value, err error) {
	defer env.StartExtensiveTracingSpanFromRoot(trace.FVMEnvDecodeArgument).End()

	v, err = env.Context().Codec.Decode(env, b)

	if err != nil {
		err = fmt.Errorf("decoding argument failed: %w", errors.NewInvalidArgumentErrorf("argument is not decodable: %w", err))
	}

	return
}
func (env *commonEnv) Hash(
	data []byte,
	tag string,
	hashAlgorithm runtime.HashAlgorithm,
) ([]byte, error) {
	defer env.StartSpanFromRoot(trace.FVMEnvHash).End()

	err := env.Meter(meter.ComputationKindHash, 1)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
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

func (env *commonEnv) Reset() {
	env.ContractUpdater.Reset()
	env.EventEmitter.Reset()
	env.AccountFreezer.Reset()
}

func (commonEnv) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}
