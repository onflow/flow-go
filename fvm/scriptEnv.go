package fvm

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/onflow/atree"
	traceLog "github.com/opentracing/opentracing-go/log"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/meter/weighted"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
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
	metrics := handler.NewMetricsHandler(fvmContext.Metrics)

	env := &ScriptEnv{
		commonEnv: commonEnv{
			ctx:           fvmContext,
			sth:           sth,
			vm:            vm,
			programs:      programsHandler,
			accounts:      accounts,
			accountKeys:   accountKeys,
			uuidGenerator: uuidGenerator,
			logs:          nil,
			rng:           nil,
			metrics:       metrics,
		},
		reqContext: reqContext,
	}

	// TODO(patrick): remove this hack after merging #2800
	env.ComputationMeter = env

	env.contracts = handler.NewContractHandler(
		accounts,
		func() bool { return true },
		func() bool { return true },
		func() []common.Address { return []common.Address{} },
		func() []common.Address { return []common.Address{} },
		func(address runtime.Address, code []byte) (bool, error) { return false, nil })

	if fvmContext.BlockHeader != nil {
		env.seedRNG(fvmContext.BlockHeader)
	}

	var err error
	// set the execution parameters from the state
	if fvmContext.AllowContextOverrideByExecutionState {
		err = env.setExecutionParameters()
	}

	return env, err
}

func (e *ScriptEnv) setExecutionParameters() error {
	// Check that the service account exists because all the settings are stored in it
	serviceAddress := e.Context().Chain.ServiceAddress()
	service := runtime.Address(serviceAddress)

	// set the property if no error, but if the error is a fatal error then return it
	setIfOk := func(prop string, err error, setter func()) (fatal error) {
		err, fatal = errors.SplitErrorTypes(err)
		if fatal != nil {
			// this is a fatal error. return it
			e.ctx.Logger.
				Error().
				Err(fatal).
				Msgf("error getting %s", prop)
			return fatal
		}
		if err != nil {
			// this is a general error.
			// could be that no setting was present in the state,
			// or that the setting was not parseable,
			// or some other deterministic thing.
			e.ctx.Logger.
				Debug().
				Err(err).
				Msgf("could not set %s. Using defaults", prop)
			return nil
		}
		// everything is ok. do the setting
		setter()
		return nil
	}

	var ok bool
	var m *weighted.Meter
	// only set the weights if the meter is a weighted.Meter
	if m, ok = e.sth.State().Meter().(*weighted.Meter); !ok {
		return nil
	}

	computationWeights, err := GetExecutionEffortWeights(e, service)
	err = setIfOk(
		"execution effort weights",
		err,
		func() { m.SetComputationWeights(computationWeights) })
	if err != nil {
		return err
	}

	memoryWeights, err := GetExecutionMemoryWeights(e, service)
	err = setIfOk(
		"execution memory weights",
		err,
		func() { m.SetMemoryWeights(memoryWeights) })
	if err != nil {
		return err
	}

	memoryLimit, err := GetExecutionMemoryLimit(e, service)
	err = setIfOk(
		"execution memory limit",
		err,
		func() { m.SetTotalMemoryLimit(memoryLimit) })
	if err != nil {
		return err
	}

	return nil
}

func (e *ScriptEnv) GetValue(owner, key []byte) ([]byte, error) {
	var valueByteSize int
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetValue)
		defer func() {
			sp.LogFields(
				traceLog.String("owner", hex.EncodeToString(owner)),
				traceLog.String("key", string(key)),
				traceLog.Int("valueByteSize", valueByteSize),
			)
			sp.Finish()
		}()
	}

	v, err := e.accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	valueByteSize = len(v)

	err = e.Meter(meter.ComputationKindGetValue, uint(valueByteSize))
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	return v, nil
}

// TODO disable SetValue for scripts, right now the view changes are discarded
func (e *ScriptEnv) SetValue(owner, key, value []byte) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvSetValue)
		sp.LogFields(
			traceLog.String("owner", hex.EncodeToString(owner)),
			traceLog.String("key", string(key)),
		)
		defer sp.Finish()
	}
	err := e.Meter(meter.ComputationKindSetValue, uint(len(value)))
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}

	err = e.accounts.SetValue(
		flow.BytesToAddress(owner),
		string(key),
		value,
	)
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}
	return nil
}

func (e *ScriptEnv) ValueExists(owner, key []byte) (exists bool, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvValueExists)
		defer sp.Finish()
	}

	err = e.Meter(meter.ComputationKindValueExists, 1)
	if err != nil {
		return false, fmt.Errorf("check value existence failed: %w", err)
	}

	v, err := e.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("check value existence failed: %w", err)
	}

	return len(v) > 0, nil
}

func (e *ScriptEnv) GetStorageUsed(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetStorageUsed)
		defer sp.Finish()
	}

	err = e.Meter(meter.ComputationKindGetStorageUsed, 1)
	if err != nil {
		return value, fmt.Errorf("get storage used failed: %w", err)
	}

	value, err = e.accounts.GetStorageUsed(flow.Address(address))
	if err != nil {
		return value, fmt.Errorf("get storage used failed: %w", err)
	}

	return value, nil
}

func (e *ScriptEnv) GetStorageCapacity(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetStorageCapacity)
		defer sp.Finish()
	}

	err = e.Meter(meter.ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return 0, fmt.Errorf("get storage capacity failed: %w", err)
	}

	result, invokeErr := InvokeAccountStorageCapacityContract(
		e,
		e.traceSpan,
		address)
	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}

	// Return type is actually a UFix64 with the unit of megabytes so some conversion is necessary
	// divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return storageMBUFixToBytesUInt(result), nil
}

func (e *ScriptEnv) GetAccountBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.Finish()
	}

	err = e.Meter(meter.ComputationKindGetAccountBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account balance failed: %w", err)
	}

	result, invokeErr := InvokeAccountBalanceContract(e, e.traceSpan, address)
	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}
	return result.ToGoValue().(uint64), nil
}

func (e *ScriptEnv) GetAccountAvailableBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.Finish()
	}

	err = e.Meter(meter.ComputationKindGetAccountAvailableBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account available balance failed: %w", err)
	}

	result, invokeErr := InvokeAccountAvailableBalanceContract(
		e,
		e.traceSpan,
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
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvResolveLocation)
		defer sp.Finish()
	}

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

func (e *ScriptEnv) GetAccountContractNames(address runtime.Address) ([]string, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountContractNames)
		defer sp.Finish()
	}

	err := e.Meter(meter.ComputationKindGetAccountContractNames, 1)
	if err != nil {
		return nil, fmt.Errorf("get account contract names failed: %w", err)
	}

	a := flow.Address(address)

	freezeError := e.accounts.CheckAccountNotFrozen(a)
	if freezeError != nil {
		return nil, fmt.Errorf("get account contract names failed: %w", freezeError)
	}

	return e.accounts.GetContractNames(a)
}

func (e *ScriptEnv) GetCode(location runtime.Location) ([]byte, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetCode)
		defer sp.Finish()
	}

	err := e.Meter(meter.ComputationKindGetCode, 1)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, errors.NewInvalidLocationErrorf(location, "expecting an AddressLocation, but other location types are passed")
	}

	address := flow.Address(contractLocation.Address)

	err = e.accounts.CheckAccountNotFrozen(address)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	add, err := e.contracts.GetContract(contractLocation.Address, contractLocation.Name)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	return add, nil
}

func (e *ScriptEnv) GetProgram(location common.Location) (*interpreter.Program, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetProgram)
		defer sp.Finish()
	}

	err := e.Meter(meter.ComputationKindGetProgram, 1)
	if err != nil {
		return nil, fmt.Errorf("get program failed: %w", err)
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		address := flow.Address(addressLocation.Address)

		freezeError := e.accounts.CheckAccountNotFrozen(address)
		if freezeError != nil {
			return nil, fmt.Errorf("get program failed: %w", freezeError)
		}
	}

	program, has := e.programs.Get(location)
	if has {
		return program, nil
	}

	return nil, nil
}

func (e *ScriptEnv) SetProgram(location common.Location, program *interpreter.Program) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvSetProgram)
		defer sp.Finish()
	}

	err := e.Meter(meter.ComputationKindSetProgram, 1)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}

	err = e.programs.Set(location, program)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}
	return nil
}

func (e *ScriptEnv) ProgramLog(message string) error {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvProgramLog)
		defer sp.Finish()
	}

	if e.ctx.CadenceLoggingEnabled {
		e.logs = append(e.logs, message)
	}
	return nil
}

func (e *ScriptEnv) Logs() []string {
	return e.logs
}

func (e *ScriptEnv) EmitEvent(_ cadence.Event) error {
	return errors.NewOperationNotSupportedError("EmitEvent")
}

func (e *ScriptEnv) Events() []flow.Event {
	return []flow.Event{}
}

func (e *ScriptEnv) GenerateUUID() (uint64, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGenerateUUID)
		defer sp.Finish()
	}

	if e.uuidGenerator == nil {
		return 0, errors.NewOperationNotSupportedError("GenerateUUID")
	}

	err := e.Meter(meter.ComputationKindGenerateUUID, 1)
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}

	uuid, err := e.uuidGenerator.GenerateUUID()
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}
	return uuid, err
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

	if e.sth.EnforceComputationLimits {
		return e.sth.State().MeterComputation(kind, intensity)
	}
	return nil
}

func (e *ScriptEnv) MeterComputation(kind common.ComputationKind, intensity uint) error {
	return e.Meter(kind, intensity)
}

func (e *ScriptEnv) ComputationUsed() uint64 {
	return uint64(e.sth.State().TotalComputationUsed())
}

func (e *ScriptEnv) meterMemory(kind common.MemoryKind, intensity uint) error {
	if e.sth.EnforceMemoryLimits() {
		return e.sth.State().MeterMemory(kind, intensity)
	}
	return nil
}

func (e *ScriptEnv) MeterMemory(usage common.MemoryUsage) error {
	return e.meterMemory(usage.Kind, uint(usage.Amount))
}

func (e *ScriptEnv) MemoryEstimate() uint64 {
	return uint64(e.sth.State().TotalMemoryEstimate())
}

func (e *ScriptEnv) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvDecodeArgument)
		defer sp.Finish()
	}

	v, err := jsoncdc.Decode(e, b)
	if err != nil {
		err = errors.NewInvalidArgumentErrorf("argument is not json decodable: %w", err)
		return nil, fmt.Errorf("decodeing argument failed: %w", err)
	}

	return v, err
}

func (e *ScriptEnv) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvVerifySignature)
		defer sp.Finish()
	}

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

// GetCurrentBlockHeight returns the current block height.
func (e *ScriptEnv) GetCurrentBlockHeight() (uint64, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetCurrentBlockHeight)
		defer sp.Finish()
	}

	err := e.Meter(meter.ComputationKindGetCurrentBlockHeight, 1)
	if err != nil {
		return 0, fmt.Errorf("get current block height failed: %w", err)
	}

	if e.ctx.BlockHeader == nil {
		return 0, errors.NewOperationNotSupportedError("GetCurrentBlockHeight")
	}
	return e.ctx.BlockHeader.Height, nil
}

// UnsafeRandom returns a random uint64, where the process of random number derivation is not cryptographically
// secure.
func (e *ScriptEnv) UnsafeRandom() (uint64, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvUnsafeRandom)
		defer sp.Finish()
	}

	if e.rng == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	// TODO (ramtin) return errors this assumption that this always succeeds might not be true
	buf := make([]byte, 8)
	_, _ = e.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf), nil
}

// GetBlockAtHeight returns the block at the given height.
func (e *ScriptEnv) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetBlockAtHeight)
		defer sp.Finish()
	}

	err := e.Meter(meter.ComputationKindGetBlockAtHeight, 1)
	if err != nil {
		return runtime.Block{}, false, fmt.Errorf("get block at height failed: %w", err)
	}

	if e.ctx.Blocks == nil {
		return runtime.Block{}, false, errors.NewOperationNotSupportedError("GetBlockAtHeight")
	}

	if e.ctx.BlockHeader != nil && height == e.ctx.BlockHeader.Height {
		return runtimeBlockFromHeader(e.ctx.BlockHeader), true, nil
	}

	header, err := e.ctx.Blocks.ByHeightFrom(height, e.ctx.BlockHeader)
	// TODO (ramtin): remove dependency on storage and move this if condition to blockfinder
	if errors.Is(err, storage.ErrNotFound) {
		return runtime.Block{}, false, nil
	} else if err != nil {
		return runtime.Block{}, false, fmt.Errorf("get block at height failed for height %v: %w", height, err)
	}

	return runtimeBlockFromHeader(header), true, nil
}

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
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountKey)
		defer sp.Finish()
	}

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

func (e *ScriptEnv) GetAccountContractCode(address runtime.Address, name string) (code []byte, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountContractCode)
		defer sp.Finish()
	}

	err = e.Meter(meter.ComputationKindGetAccountContractCode, 1)
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	code, err = e.GetCode(common.AddressLocation{
		Address: address,
		Name:    name,
	})
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	return code, nil
}

func (e *ScriptEnv) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}

func (e *ScriptEnv) GetSigningAccounts() ([]runtime.Address, error) {
	return nil, errors.NewOperationNotSupportedError("GetSigningAccounts")
}

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (e *ScriptEnv) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	err := e.Meter(meter.ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}

	v, err := e.accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}
