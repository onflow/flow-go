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
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// TODO(patrick): rm after emulator is updated
type Environment = environment.Environment

var _ environment.Environment = &facadeEnvironment{}

// facadeEnvironment exposes various fvm business logic as a single interface.
type facadeEnvironment struct {
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

	programs *handler.ProgramsHandler
	accounts environment.Accounts
}

func newFacadeEnvironment(
	ctx Context,
	stateTransaction *state.StateHolder,
	programs handler.TransactionPrograms,
	tracer *environment.Tracer,
	meter environment.Meter,
) *facadeEnvironment {
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

	env := &facadeEnvironment{
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
		programs:        programsHandler,
		accounts:        accounts,

		TransactionInfo:   environment.NoTransactionInfo{},
		EventEmitter:      environment.NoEventEmitter{},
		AccountCreator:    environment.NoAccountCreator{},
		AccountFreezer:    environment.NoAccountFreezer{},
		ContractUpdater:   handler.NoContractUpdater{},
		AccountKeyUpdater: handler.NoAccountKeyUpdater{},
	}

	env.Runtime.SetEnvironment(env)

	return env
}

func (env *facadeEnvironment) GetProgram(location common.Location) (*interpreter.Program, error) {
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

func (env *facadeEnvironment) SetProgram(location common.Location, program *interpreter.Program) error {
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

func (env *facadeEnvironment) DecodeArgument(b []byte, _ cadence.Type) (cadence.Value, error) {
	defer env.StartExtensiveTracingSpanFromRoot(trace.FVMEnvDecodeArgument).End()

	v, err := jsoncdc.Decode(env, b)
	if err != nil {
		err = errors.NewInvalidArgumentErrorf("argument is not json decodable: %w", err)
		return nil, fmt.Errorf("decodeing argument failed: %w", err)
	}

	return v, err
}

func (env *facadeEnvironment) FlushPendingUpdates() (
	programs.ModifiedSetsInvalidator,
	error,
) {
	keys, err := env.ContractUpdater.Commit()
	return programs.ModifiedSetsInvalidator{
		ContractUpdateKeys: keys,
		FrozenAccounts:     env.FrozenAccounts(),
	}, err
}

func (env *facadeEnvironment) Reset() {
	env.ContractUpdater.Reset()
	env.EventEmitter.Reset()
	env.AccountFreezer.Reset()
}

func (facadeEnvironment) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}
