package fvm

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/onflow/atree"
	"github.com/opentracing/opentracing-go"
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
	ctx           Context
	sth           *state.StateHolder
	vm            *VirtualMachine
	accounts      state.Accounts
	contracts     *handler.ContractHandler
	programs      *handler.ProgramsHandler
	accountKeys   *handler.AccountKeyHandler
	metrics       *handler.MetricsHandler
	uuidGenerator *state.UUIDGenerator
	logs          []string
	rng           *rand.Rand
	traceSpan     opentracing.Span
	reqContext    context.Context
}

func (e *ScriptEnv) Context() *Context {
	return &e.ctx
}

func (e *ScriptEnv) VM() *VirtualMachine {
	return e.vm
}

func NewScriptEnvironment(
	reqContext context.Context,
	fvmContext Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs,
) *ScriptEnv {

	accounts := state.NewAccounts(sth)
	uuidGenerator := state.NewUUIDGenerator(sth)
	programsHandler := handler.NewProgramsHandler(programs, sth)
	accountKeys := handler.NewAccountKeyHandler(accounts)
	metrics := handler.NewMetricsHandler(fvmContext.Metrics)

	env := &ScriptEnv{
		ctx:           fvmContext,
		sth:           sth,
		vm:            vm,
		metrics:       metrics,
		accounts:      accounts,
		accountKeys:   accountKeys,
		uuidGenerator: uuidGenerator,
		programs:      programsHandler,
		reqContext:    reqContext,
	}

	env.contracts = handler.NewContractHandler(
		accounts,
		true,
		func() []common.Address { return []common.Address{} },
		func(address runtime.Address, code []byte) (bool, error) { return false, nil })

	if fvmContext.BlockHeader != nil {
		env.seedRNG(fvmContext.BlockHeader)
	}

	env.setMeteringWeights()

	return env
}

func (e *ScriptEnv) setMeteringWeights() {
	var m *weighted.Meter
	var ok bool
	// only set the weights if the meter is a weighted.Meter
	if m, ok = e.sth.State().Meter().(*weighted.Meter); !ok {
		return
	}

	computationWeights, memoryWeights, err := getExecutionWeights(e, e.accounts)

	if err != nil {
		e.ctx.Logger.
			Info().
			Err(err).
			Msg("could not set execution weights. Using defaults")
		return
	}

	m.SetComputationWeights(computationWeights)
	m.SetMemoryWeights(memoryWeights)
}

func (e *ScriptEnv) ResourceOwnerChanged(_ *interpreter.CompositeValue, _ common.Address, _ common.Address) {
}

func (e *ScriptEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	e.rng = rand.New(source)
}

func (e *ScriptEnv) isTraceable() bool {
	return e.ctx.Tracer != nil && e.traceSpan != nil
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

	err = e.meterComputation(meter.ComputationKindGetValue, uint(valueByteSize))
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
	err := e.meterComputation(meter.ComputationKindSetValue, uint(len(value)))
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

	err = e.meterComputation(meter.ComputationKindValueExists, 1)
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

	err = e.meterComputation(meter.ComputationKindGetStorageUsed, 1)
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

	err = e.meterComputation(meter.ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return 0, fmt.Errorf("get storage capacity failed: %w", err)
	}

	accountStorageCapacity := AccountStorageCapacityInvocation(e, e.traceSpan)
	result, invokeErr := accountStorageCapacity(address)

	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, storage capacity will be 0.
	// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// 2. There will also be an error in case the accounts balance times megabytesPerFlow constant overflows,
	//		which shouldn't happen unless the the price of storage is reduced at least 100 fold
	// 3. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	if invokeErr != nil {
		return 0, nil
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

	err = e.meterComputation(meter.ComputationKindGetAccountBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account balance failed: %w", err)
	}

	accountBalance := AccountBalanceInvocation(e, e.traceSpan)
	result, invokeErr := accountBalance(address)

	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, balance will be 0.
	if invokeErr != nil {
		return 0, nil
	}
	return result.ToGoValue().(uint64), nil
}

func (e *ScriptEnv) GetAccountAvailableBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindGetAccountAvailableBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account available balance failed: %w", err)
	}

	accountAvailableBalance := AccountAvailableBalanceInvocation(e, e.traceSpan)
	result, invokeErr := accountAvailableBalance(address)

	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, available balance will be 0.
	// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// 2. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	if invokeErr != nil {
		return 0, nil
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

	err := e.meterComputation(meter.ComputationKindResolveLocation, 1)
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

	err := e.meterComputation(meter.ComputationKindGetAccountContractNames, 1)
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

	err := e.meterComputation(meter.ComputationKindGetCode, 1)
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

	err := e.meterComputation(meter.ComputationKindGetProgram, 1)
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

	err := e.meterComputation(meter.ComputationKindSetProgram, 1)
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

	err := e.meterComputation(meter.ComputationKindGenerateUUID, 1)
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

func (e *ScriptEnv) meterComputation(kind common.ComputationKind, intensity uint) error {
	// this method is called on every unit of operation, so
	// checking the context here is the most likely would capture
	// timeouts or cancellation as soon as they happen, though
	// we might revisit this when optimizing script execution
	// by only checking on specific kind of meterComputation calls.
	if err := e.checkContext(); err != nil {
		return err
	}

	if e.sth.EnforceComputationLimits {
		return e.sth.State().MeterComputation(kind, intensity)
	}
	return nil
}

func (e *ScriptEnv) MeterComputation(kind common.ComputationKind, intensity uint) error {
	return e.meterComputation(kind, intensity)
}

func (e *ScriptEnv) ComputationUsed() uint64 {
	return uint64(e.sth.State().TotalComputationUsed())
}

func (e *ScriptEnv) DecodeArgument(b []byte, _ cadence.Type) (cadence.Value, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvDecodeArgument)
		defer sp.Finish()
	}

	v, err := jsoncdc.Decode(b)
	if err != nil {
		err = errors.NewInvalidArgumentErrorf("argument is not json decodable: %w", err)
		return nil, fmt.Errorf("decodeing argument failed: %w", err)
	}

	return v, err
}

func (e *ScriptEnv) Hash(data []byte, tag string, hashAlgorithm runtime.HashAlgorithm) ([]byte, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvHash)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindHash, 1)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
	}

	hashAlgo := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	return crypto.HashWithTag(hashAlgo, tag, data)
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

	err := e.meterComputation(meter.ComputationKindVerifySignature, 1)
	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	valid, err := crypto.VerifySignatureFromRuntime(
		e.ctx.SignatureVerifier,
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
	err := e.meterComputation(meter.ComputationKindValidatePublicKey, 1)
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

	err := e.meterComputation(meter.ComputationKindGetCurrentBlockHeight, 1)
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

	err := e.meterComputation(meter.ComputationKindGetBlockAtHeight, 1)
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

	err := e.meterComputation(meter.ComputationKindGetAccountKey, 1)
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

	err = e.meterComputation(meter.ComputationKindGetAccountContractCode, 1)
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

func (e *ScriptEnv) ImplementationDebugLog(message string) error {
	e.ctx.Logger.Debug().Msgf("Cadence: %s", message)
	return nil
}

func (e *ScriptEnv) RecordTrace(operation string, location common.Location, duration time.Duration, logs []opentracing.LogRecord) {
	if !e.isTraceable() {
		return
	}
	if location != nil {
		if logs == nil {
			logs = make([]opentracing.LogRecord, 0)
		}
		logs = append(logs, opentracing.LogRecord{Timestamp: time.Now(),
			Fields: []traceLog.Field{traceLog.String("location", location.String())},
		})
	}
	spanName := trace.FVMCadenceTrace.Child(operation)
	e.ctx.Tracer.RecordSpanFromParent(e.traceSpan, spanName, duration, logs)
}

func (e *ScriptEnv) ProgramParsed(location common.Location, duration time.Duration) {
	e.RecordTrace("parseProgram", location, duration, nil)
	e.metrics.ProgramParsed(location, duration)
}

func (e *ScriptEnv) ProgramChecked(location common.Location, duration time.Duration) {
	e.RecordTrace("checkProgram", location, duration, nil)
	e.metrics.ProgramChecked(location, duration)
}

func (e *ScriptEnv) ProgramInterpreted(location common.Location, duration time.Duration) {
	e.RecordTrace("interpretProgram", location, duration, nil)
	e.metrics.ProgramInterpreted(location, duration)
}

func (e *ScriptEnv) ValueEncoded(duration time.Duration) {
	e.RecordTrace("encodeValue", nil, duration, nil)
	e.metrics.ValueEncoded(duration)
}

func (e *ScriptEnv) ValueDecoded(duration time.Duration) {
	e.RecordTrace("decodeValue", nil, duration, nil)
	e.metrics.ValueDecoded(duration)
}

// Commit commits changes and return a list of updated keys
func (e *ScriptEnv) Commit() ([]programs.ContractUpdateKey, error) {
	// commit changes and return a list of updated keys
	err := e.programs.Cleanup()
	if err != nil {
		return nil, err
	}
	return e.contracts.Commit()
}

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (e *ScriptEnv) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	err := e.meterComputation(meter.ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}

	v, err := e.accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}

func (e *ScriptEnv) BLSVerifyPOP(pk *runtime.PublicKey, sig []byte) (bool, error) {
	return crypto.VerifyPOP(pk, sig)
}

func (e *ScriptEnv) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	return crypto.AggregateSignatures(sigs)
}

func (e *ScriptEnv) BLSAggregatePublicKeys(keys []*runtime.PublicKey) (*runtime.PublicKey, error) {
	return crypto.AggregatePublicKeys(keys)
}
