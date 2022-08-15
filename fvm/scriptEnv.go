package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var _ runtime.Interface = &ScriptEnv{}
var _ Environment = &ScriptEnv{}

// ScriptEnv is a read-only mostly used for executing scripts.
type ScriptEnv struct {
	commonEnv

	reqContext context.Context
}

func NewScriptEnvironment(
	reqContext context.Context,
	fvmContext Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs,
) (*ScriptEnv, error) {

	accounts := state.NewAccounts(sth)
	uuidGenerator := state.NewUUIDGenerator(sth)
	programsHandler := handler.NewProgramsHandler(programs, sth)
	accountKeys := handler.NewAccountKeyHandler(accounts)
	tracer := NewTracer(fvmContext.Tracer, nil, fvmContext.ExtensiveTracing)

	env := &ScriptEnv{
		commonEnv: commonEnv{
			Tracer: tracer,
			ProgramLogger: NewProgramLogger(
				tracer,
				fvmContext.Metrics,
				fvmContext.CadenceLoggingEnabled,
			),
			UnsafeRandomGenerator: NewUnsafeRandomGenerator(
				tracer,
				fvmContext.BlockHeader,
			),
			ctx:            fvmContext,
			sth:            sth,
			vm:             vm,
			programs:       programsHandler,
			accounts:       accounts,
			accountKeys:    accountKeys,
			uuidGenerator:  uuidGenerator,
			frozenAccounts: nil,
		},
		reqContext: reqContext,
	}

	// TODO(patrick): remove this hack
	env.MeterInterface = env
	env.AccountInterface = env
	env.fullEnv = env

	env.contracts = handler.NewContractHandler(
		accounts,
		func() bool { return true },
		func() bool { return true },
		func() []common.Address { return []common.Address{} },
		func() []common.Address { return []common.Address{} },
		func(address runtime.Address, code []byte) (bool, error) { return false, nil })

	err := setMeterParameters(&env.commonEnv)

	return env, err
}

func (e *ScriptEnv) GetStorageCapacity(address common.Address) (value uint64, err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvGetStorageCapacity).End()

	err = e.Meter(meter.ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return 0, fmt.Errorf("get storage capacity failed: %w", err)
	}

	result, invokeErr := InvokeAccountStorageCapacityContract(
		e,
		address)
	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}

	// Return type is actually a UFix64 with the unit of megabytes so some conversion is necessary
	// divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return storageMBUFixToBytesUInt(result), nil
}

func (e *ScriptEnv) GetAccountBalance(address common.Address) (value uint64, err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvGetAccountBalance).End()

	err = e.Meter(meter.ComputationKindGetAccountBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account balance failed: %w", err)
	}

	result, invokeErr := InvokeAccountBalanceContract(e, address)
	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}
	return result.ToGoValue().(uint64), nil
}

func (e *ScriptEnv) GetAccountAvailableBalance(address common.Address) (value uint64, err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvGetAccountBalance).End()

	err = e.Meter(meter.ComputationKindGetAccountAvailableBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account available balance failed: %w", err)
	}

	result, invokeErr := InvokeAccountAvailableBalanceContract(
		e,
		address)

	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}
	return result.ToGoValue().(uint64), nil
}

func (e *ScriptEnv) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	defer e.StartExtensiveTracingSpanFromRoot(trace.FVMEnvResolveLocation).End()

	err := e.Meter(meter.ComputationKindResolveLocation, 1)
	if err != nil {
		return nil, fmt.Errorf("resolve location failed: %w", err)
	}

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.
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

		err := e.accounts.CheckAccountNotFrozen(address)
		if err != nil {
			return nil, fmt.Errorf("resolve location failed: %w", err)
		}

		contractNames, err := e.contracts.GetContractNames(addressLocation.Address)
		if err != nil {
			return nil, fmt.Errorf("resolve location failed: %w", err)
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

func (e *ScriptEnv) EmitEvent(_ cadence.Event) error {
	return errors.NewOperationNotSupportedError("EmitEvent")
}

func (e *ScriptEnv) Events() []flow.Event {
	return []flow.Event{}
}

func (e *ScriptEnv) checkContext() error {
	// in the future this context check should be done inside the cadence
	select {
	case <-e.reqContext.Done():
		err := e.reqContext.Err()
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.NewScriptExecutionTimedOutError()
		}
		return errors.NewScriptExecutionCancelledError(err)
	default:
		return nil
	}
}

func (e *ScriptEnv) Meter(kind common.ComputationKind, intensity uint) error {
	// this method is called on every unit of operation, so
	// checking the context here is the most likely would capture
	// timeouts or cancellation as soon as they happen, though
	// we might revisit this when optimizing script execution
	// by only checking on specific kind of Meter calls.
	if err := e.checkContext(); err != nil {
		return err
	}

	if e.sth.EnforceComputationLimits() {
		return e.sth.MeterComputation(kind, intensity)
	}
	return nil
}

func (e *ScriptEnv) meterMemory(kind common.MemoryKind, intensity uint) error {
	if e.sth.EnforceMemoryLimits() {
		return e.sth.MeterMemory(kind, intensity)
	}
	return nil
}

func (e *ScriptEnv) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	defer e.StartSpanFromRoot(trace.FVMEnvVerifySignature).End()

	err := e.Meter(meter.ComputationKindVerifySignature, 1)
	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	valid, err := crypto.VerifySignatureFromRuntime(
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm,
	)

	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	return valid, nil
}

func (e *ScriptEnv) ValidatePublicKey(pk *runtime.PublicKey) error {
	err := e.Meter(meter.ComputationKindValidatePublicKey, 1)
	if err != nil {
		return fmt.Errorf("validate public key failed: %w", err)
	}

	return crypto.ValidatePublicKey(pk.SignAlgo, pk.PublicKey)
}

// Block Environment Functions

func (e *ScriptEnv) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	return runtime.Address{}, errors.NewOperationNotSupportedError("CreateAccount")
}

func (e *ScriptEnv) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	return errors.NewOperationNotSupportedError("AddEncodedAccountKey")
}

func (e *ScriptEnv) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	return nil, errors.NewOperationNotSupportedError("RevokeEncodedAccountKey")
}

func (e *ScriptEnv) AddAccountKey(_ runtime.Address, _ *runtime.PublicKey, _ runtime.HashAlgorithm, _ int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("AddAccountKey")
}

func (e *ScriptEnv) GetAccountKey(address runtime.Address, index int) (*runtime.AccountKey, error) {
	defer e.StartSpanFromRoot(trace.FVMEnvGetAccountKey).End()

	err := e.Meter(meter.ComputationKindGetAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}

	if e.accountKeys != nil {
		accKey, err := e.accountKeys.GetAccountKey(address, index)
		if err != nil {
			return nil, fmt.Errorf("get account key failed: %w", err)
		}
		return accKey, err
	}

	return nil, errors.NewOperationNotSupportedError("GetAccountKey")
}

func (e *ScriptEnv) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("RevokeAccountKey")
}

func (e *ScriptEnv) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	return errors.NewOperationNotSupportedError("UpdateAccountContractCode")
}

func (e *ScriptEnv) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}

func (e *ScriptEnv) GetSigningAccounts() ([]runtime.Address, error) {
	return nil, errors.NewOperationNotSupportedError("GetSigningAccounts")
}
