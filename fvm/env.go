package fvm

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
)

// Environment accepts a context and a virtual machine instance and provides
// cadence runtime interface methods to the runtime.
type Environment interface {
	Context() *Context
	VM() *VirtualMachine
	runtime.Interface
}

// TODO(patrick): refactor this into an object.
// TODO(patrick): rename the method names to match meter.Meter interface.
type MeterInterface interface {
	Meter(common.ComputationKind, uint) error

	meterMemory(kind common.MemoryKind, intensity uint) error
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

	// TODO(patrick): figure out where this belongs
	GetSigningAccounts() ([]runtime.Address, error)
}

// Parts of the environment that are common to all transaction and script
// executions.
type commonEnv struct {
	MeterInterface
	AccountInterface

	ctx           Context
	sth           *state.StateHolder
	vm            *VirtualMachine
	programs      *handler.ProgramsHandler
	accounts      state.Accounts
	accountKeys   *handler.AccountKeyHandler
	contracts     *handler.ContractHandler
	uuidGenerator *state.UUIDGenerator
	metrics       *handler.MetricsHandler
	logs          []string
	rng           *rand.Rand
	traceSpan     opentracing.Span
}

// TODO(patrick): rm once Meter object has been refactored
func (env *commonEnv) MeterComputation(kind common.ComputationKind, intensity uint) error {
	return env.Meter(kind, intensity)
}

// TODO(patrick): rm once Meter object has been refactored
func (env *commonEnv) ComputationUsed() uint64 {
	return uint64(env.sth.State().TotalComputationUsed())
}

// TODO(patrick): rm once Meter object has been refactored
func (env *commonEnv) MeterMemory(usage common.MemoryUsage) error {
	return env.meterMemory(usage.Kind, uint(usage.Amount))
}

// TODO(patrick): rm once Meter object has been refactored
func (env *commonEnv) MemoryEstimate() uint64 {
	return uint64(env.sth.State().TotalMemoryEstimate())
}

func (env *commonEnv) Context() *Context {
	return &env.ctx
}

func (env *commonEnv) VM() *VirtualMachine {
	return env.vm
}

func (env *commonEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block
	// header ID. The random number generator will be used by the UnsafeRandom
	// function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	env.rng = rand.New(source)
}

// UnsafeRandom returns a random uint64, where the process of random number derivation is not cryptographically
// secure.
func (env *commonEnv) UnsafeRandom() (uint64, error) {
	if env.isTraceable() && env.ctx.ExtensiveTracing {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvUnsafeRandom)
		defer sp.Finish()
	}

	if env.rng == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	// TODO (ramtin) return errors this assumption that this always succeeds might not be true
	buf := make([]byte, 8)
	_, _ = env.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf), nil
}

func (env *commonEnv) isTraceable() bool {
	return env.ctx.Tracer != nil && env.traceSpan != nil
}

func (env *commonEnv) GenerateUUID() (uint64, error) {
	if env.isTraceable() && env.ctx.ExtensiveTracing {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGenerateUUID)
		defer sp.Finish()
	}

	if env.uuidGenerator == nil {
		return 0, errors.NewOperationNotSupportedError("GenerateUUID")
	}

	err := env.Meter(meter.ComputationKindGenerateUUID, 1)
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}

	uuid, err := env.uuidGenerator.GenerateUUID()
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}
	return uuid, err
}

// GetCurrentBlockHeight returns the current block height.
func (env *commonEnv) GetCurrentBlockHeight() (uint64, error) {
	if env.isTraceable() && env.ctx.ExtensiveTracing {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetCurrentBlockHeight)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindGetCurrentBlockHeight, 1)
	if err != nil {
		return 0, fmt.Errorf("get current block height failed: %w", err)
	}

	if env.ctx.BlockHeader == nil {
		return 0, errors.NewOperationNotSupportedError("GetCurrentBlockHeight")
	}
	return env.ctx.BlockHeader.Height, nil
}

// GetBlockAtHeight returns the block at the given height.
func (env *commonEnv) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetBlockAtHeight)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindGetBlockAtHeight, 1)
	if err != nil {
		return runtime.Block{}, false, fmt.Errorf("get block at height failed: %w", err)
	}

	if env.ctx.Blocks == nil {
		return runtime.Block{}, false, errors.NewOperationNotSupportedError("GetBlockAtHeight")
	}

	if env.ctx.BlockHeader != nil && height == env.ctx.BlockHeader.Height {
		return runtimeBlockFromHeader(env.ctx.BlockHeader), true, nil
	}

	header, err := env.ctx.Blocks.ByHeightFrom(height, env.ctx.BlockHeader)
	// TODO (ramtin): remove dependency on storage and move this if condition to blockfinder
	if errors.Is(err, storage.ErrNotFound) {
		return runtime.Block{}, false, nil
	} else if err != nil {
		return runtime.Block{}, false, fmt.Errorf("get block at height failed for height %v: %w", height, err)
	}

	return runtimeBlockFromHeader(header), true, nil
}

func (env *commonEnv) GetValue(owner, key []byte) ([]byte, error) {
	var valueByteSize int
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetValue)
		defer func() {
			sp.LogFields(
				traceLog.String("owner", hex.EncodeToString(owner)),
				traceLog.String("key", string(key)),
				traceLog.Int("valueByteSize", valueByteSize),
			)
			sp.Finish()
		}()
	}

	v, err := env.accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	valueByteSize = len(v)

	err = env.Meter(meter.ComputationKindGetValue, uint(valueByteSize))
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	return v, nil
}

// TODO disable SetValue for scripts, right now the view changes are discarded
func (env *commonEnv) SetValue(owner, key, value []byte) error {
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvSetValue)
		sp.LogFields(
			traceLog.String("owner", hex.EncodeToString(owner)),
			traceLog.String("key", string(key)),
		)
		defer sp.Finish()
	}
	err := env.Meter(meter.ComputationKindSetValue, uint(len(value)))
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
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvValueExists)
		defer sp.Finish()
	}

	err = env.Meter(meter.ComputationKindValueExists, 1)
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
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetStorageUsed)
		defer sp.Finish()
	}

	err = env.Meter(meter.ComputationKindGetStorageUsed, 1)
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

func (env *commonEnv) GetAccountContractNames(address runtime.Address) ([]string, error) {
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetAccountContractNames)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindGetAccountContractNames, 1)
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

func (env *commonEnv) GetCode(location runtime.Location) ([]byte, error) {
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetCode)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindGetCode, 1)
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
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetAccountContractCode)
		defer sp.Finish()
	}

	err = env.Meter(meter.ComputationKindGetAccountContractCode, 1)
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
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvGetProgram)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindGetProgram, 1)
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
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvSetProgram)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindSetProgram, 1)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}

	err = env.programs.Set(location, program)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}
	return nil
}

func (env *commonEnv) ImplementationDebugLog(message string) error {
	env.ctx.Logger.Debug().Msgf("Cadence: %s", message)
	return nil
}

func (env *commonEnv) Hash(
	data []byte,
	tag string,
	hashAlgorithm runtime.HashAlgorithm,
) ([]byte, error) {
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvHash)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindHash, 1)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
	}

	hashAlgo := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	return crypto.HashWithTag(hashAlgo, tag, data)
}

func (env *commonEnv) DecodeArgument(b []byte, _ cadence.Type) (cadence.Value, error) {
	if env.isTraceable() && env.ctx.ExtensiveTracing {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvDecodeArgument)
		defer sp.Finish()
	}

	v, err := jsoncdc.Decode(env, b)
	if err != nil {
		err = errors.NewInvalidArgumentErrorf("argument is not json decodable: %w", err)
		return nil, fmt.Errorf("decodeing argument failed: %w", err)
	}

	return v, err
}

func (env *commonEnv) RecordTrace(operation string, location common.Location, duration time.Duration, logs []opentracing.LogRecord) {
	if !env.isTraceable() {
		return
	}
	if location != nil {
		if logs == nil {
			logs = make([]opentracing.LogRecord, 0, 1)
		}
		logs = append(logs, opentracing.LogRecord{Timestamp: time.Now(),
			Fields: []traceLog.Field{traceLog.String("location", location.String())},
		})
	}
	spanName := trace.FVMCadenceTrace.Child(operation)
	env.ctx.Tracer.RecordSpanFromParent(env.traceSpan, spanName, duration, logs)
}

func (env *commonEnv) ProgramParsed(location common.Location, duration time.Duration) {
	env.RecordTrace("parseProgram", location, duration, nil)
	env.metrics.ProgramParsed(location, duration)
}

func (env *commonEnv) ProgramChecked(location common.Location, duration time.Duration) {
	env.RecordTrace("checkProgram", location, duration, nil)
	env.metrics.ProgramChecked(location, duration)
}

func (env *commonEnv) ProgramInterpreted(location common.Location, duration time.Duration) {
	env.RecordTrace("interpretProgram", location, duration, nil)
	env.metrics.ProgramInterpreted(location, duration)
}

func (env *commonEnv) ValueEncoded(duration time.Duration) {
	env.RecordTrace("encodeValue", nil, duration, nil)
	env.metrics.ValueEncoded(duration)
}

func (env *commonEnv) ValueDecoded(duration time.Duration) {
	env.RecordTrace("decodeValue", nil, duration, nil)
	env.metrics.ValueDecoded(duration)
}

// Commit commits changes and return a list of updated keys
func (env *commonEnv) Commit() ([]programs.ContractUpdateKey, error) {
	// commit changes and return a list of updated keys
	err := env.programs.Cleanup()
	if err != nil {
		return nil, err
	}
	return env.contracts.Commit()
}

func (commonEnv) BLSVerifyPOP(pk *runtime.PublicKey, sig []byte) (bool, error) {
	return crypto.VerifyPOP(pk, sig)
}

func (commonEnv) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	return crypto.AggregateSignatures(sigs)
}

func (commonEnv) BLSAggregatePublicKeys(
	keys []*runtime.PublicKey,
) (*runtime.PublicKey, error) {

	return crypto.AggregatePublicKeys(keys)
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
	err := env.Meter(meter.ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("allocate storage index failed: %w", err)
	}

	v, err := env.accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}
