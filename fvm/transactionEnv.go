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
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/meter/weighted"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
)

var _ runtime.Interface = &TransactionEnv{}

// TransactionEnv is a read-write environment used for executing flow transactions.
type TransactionEnv struct {
	vm               *VirtualMachine
	ctx              Context
	sth              *state.StateHolder
	programs         *handler.ProgramsHandler
	accounts         state.Accounts
	uuidGenerator    *state.UUIDGenerator
	contracts        *handler.ContractHandler
	accountKeys      *handler.AccountKeyHandler
	metrics          *handler.MetricsHandler
	eventHandler     *handler.EventHandler
	addressGenerator flow.AddressGenerator
	rng              *rand.Rand
	logs             []string
	tx               *flow.TransactionBody
	txIndex          uint32
	txID             flow.Identifier
	traceSpan        opentracing.Span
	authorizers      []runtime.Address
}

func NewTransactionEnvironment(
	ctx Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan opentracing.Span,
) (*TransactionEnv, error) {

	accounts := state.NewAccounts(sth)
	generator := state.NewStateBoundAddressGenerator(sth, ctx.Chain)
	uuidGenerator := state.NewUUIDGenerator(sth)
	programsHandler := handler.NewProgramsHandler(programs, sth)
	// TODO set the flags on context
	eventHandler := handler.NewEventHandler(ctx.Chain,
		ctx.EventCollectionEnabled,
		ctx.ServiceEventCollectionEnabled,
		ctx.EventCollectionByteSizeLimit,
	)
	accountKeys := handler.NewAccountKeyHandler(accounts)
	metrics := handler.NewMetricsHandler(ctx.Metrics)

	env := &TransactionEnv{
		vm:               vm,
		ctx:              ctx,
		sth:              sth,
		metrics:          metrics,
		programs:         programsHandler,
		accounts:         accounts,
		accountKeys:      accountKeys,
		addressGenerator: generator,
		uuidGenerator:    uuidGenerator,
		eventHandler:     eventHandler,
		tx:               tx,
		txIndex:          txIndex,
		txID:             tx.ID(),
		traceSpan:        traceSpan,
	}

	env.contracts = handler.NewContractHandler(accounts,
		ctx.RestrictedDeploymentEnabled,
		env.GetAuthorizedAccountsForContractUpdates,
		env.useContractAuditVoucher,
	)

	if ctx.BlockHeader != nil {
		env.seedRNG(ctx.BlockHeader)
	}

	err := env.setMeteringWeights()

	return env, err
}

func (e *TransactionEnv) setMeteringWeights() error {
	var m *weighted.Meter
	var ok bool
	// only set the weights if the meter is a weighted.Meter
	if m, ok = e.sth.State().Meter().(*weighted.Meter); !ok {
		return nil
	}

	computationWeights, memoryWeights, err := getExecutionWeights(e, e.accounts)
	err, fatal := errors.SplitErrorTypes(err)
	if fatal != nil {
		e.ctx.Logger.
			Error().
			Err(fatal).
			Msg("error getting execution weights")
		return fatal
	}
	if err != nil {
		e.ctx.Logger.
			Info().
			Err(err).
			Msg("could not set execution weights. Using defaults")
		return nil
	}

	m.SetComputationWeights(computationWeights)
	m.SetMemoryWeights(memoryWeights)
	return nil
}

func (e *TransactionEnv) TxIndex() uint32 {
	return e.txIndex
}

func (e *TransactionEnv) TxID() flow.Identifier {
	return e.txID
}

func (e *TransactionEnv) Context() *Context {
	return &e.ctx
}

func (e *TransactionEnv) VM() *VirtualMachine {
	return e.vm
}

func (e *TransactionEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	e.rng = rand.New(source)
}

func (e *TransactionEnv) isTraceable() bool {
	return e.ctx.Tracer != nil && e.traceSpan != nil
}

// GetAuthorizedAccountsForContractUpdates returns a list of addresses that
// are authorized to update/deploy contracts
//
// It reads a storage path from service account and parse the addresses.
// if any issue occurs on the process (missing registers, stored value properly not set)
// it gracefully handle it and falls back to default behaviour (only service account be authorized)
func (e *TransactionEnv) GetAuthorizedAccountsForContractUpdates() []common.Address {
	// set default to service account only
	service := runtime.Address(e.ctx.Chain.ServiceAddress())
	defaultAccounts := []runtime.Address{service}

	value, err := e.vm.Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.ContractDeploymentAuthorizedAddressesPathDomain,
			Identifier: blueprints.ContractDeploymentAuthorizedAddressesPathIdentifier,
		},
		runtime.Context{Interface: e},
	)
	if err != nil {
		e.ctx.Logger.Warn().Msg("failed to read contract deployment authorized accounts from service account. using default behaviour instead.")
		return defaultAccounts
	}
	addresses, ok := utils.CadenceValueToAddressSlice(value)
	if !ok {
		e.ctx.Logger.Warn().Msg("failed to parse contract deployment authorized accounts from service account. using default behaviour instead.")
		return defaultAccounts
	}
	return addresses
}

func (e *TransactionEnv) useContractAuditVoucher(address runtime.Address, code []byte) (bool, error) {
	useVoucher := UseContractAuditVoucherInvocation(e, e.traceSpan)
	return useVoucher(address, string(code[:]))
}

func (e *TransactionEnv) isAuthorizerServiceAccount() bool {
	return e.isAuthorizer(runtime.Address(e.ctx.Chain.ServiceAddress()))
}

func (e *TransactionEnv) isAuthorizer(address runtime.Address) bool {
	for _, accountAddress := range e.getSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}
	return false
}

func (e *TransactionEnv) GetValue(owner, key []byte) ([]byte, error) {
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

func (e *TransactionEnv) SetValue(owner, key, value []byte) error {
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

func (e *TransactionEnv) ValueExists(owner, key []byte) (exists bool, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvValueExists)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindValueExists, 1)
	if err != nil {
		return false, fmt.Errorf("checking value existence failed: %w", err)
	}

	v, err := e.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("checking value existence failed: %w", err)
	}

	return len(v) > 0, nil
}

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (e *TransactionEnv) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	err := e.meterComputation(meter.ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("allocate storage index failed: %w", err)
	}

	v, err := e.accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}

func (e *TransactionEnv) GetStorageUsed(address common.Address) (value uint64, err error) {
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

func (e *TransactionEnv) GetStorageCapacity(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetStorageCapacity)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return value, fmt.Errorf("get storage capacity failed: %w", err)
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

	return storageMBUFixToBytesUInt(result), nil
}

// storageMBUFixToBytesUInt converts the return type of storage capacity which is a UFix64 with the unit of megabytes to
// UInt with the unit of bytes
func storageMBUFixToBytesUInt(result cadence.Value) uint64 {
	// Divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return result.ToGoValue().(uint64) / 100
}

func (e *TransactionEnv) GetAccountBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindGetAccountBalance, 1)
	if err != nil {
		return value, fmt.Errorf("get account balance failed: %w", err)
	}

	accountBalance := AccountBalanceInvocation(e, e.traceSpan)
	result, invokeErr := accountBalance(address)

	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, balance will be 0.
	if invokeErr != nil {
		return 0, nil
	}
	return result.ToGoValue().(uint64), nil
}

func (e *TransactionEnv) GetAccountAvailableBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindGetAccountAvailableBalance, 1)
	if err != nil {
		return value, fmt.Errorf("get account available balance failed: %w", err)
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

func (e *TransactionEnv) ResolveLocation(
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
			return nil, fmt.Errorf("resolving location failed: %w", err)
		}

		contractNames, err := e.contracts.GetContractNames(addressLocation.Address)
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

func (e *TransactionEnv) GetCode(location runtime.Location) ([]byte, error) {
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

func (e *TransactionEnv) GetAccountContractNames(address runtime.Address) ([]string, error) {
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

func (e *TransactionEnv) GetProgram(location common.Location) (*interpreter.Program, error) {
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

func (e *TransactionEnv) SetProgram(location common.Location, program *interpreter.Program) error {
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

func (e *TransactionEnv) ProgramLog(message string) error {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvProgramLog)
		defer sp.Finish()
	}

	if e.ctx.CadenceLoggingEnabled {
		e.logs = append(e.logs, message)
	}
	return nil
}

func (e *TransactionEnv) Logs() []string {
	return e.logs
}

func (e *TransactionEnv) EmitEvent(event cadence.Event) error {
	// only trace when extensive tracing
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvEmitEvent)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindEmitEvent, 1)
	if err != nil {
		return fmt.Errorf("emit event failed: %w", err)
	}

	return e.eventHandler.EmitEvent(event, e.txID, e.txIndex, e.tx.Payer)
}

func (e *TransactionEnv) Events() []flow.Event {
	return e.eventHandler.Events()
}

func (e *TransactionEnv) ServiceEvents() []flow.Event {
	return e.eventHandler.ServiceEvents()
}

func (e *TransactionEnv) GenerateUUID() (uint64, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGenerateUUID)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindGenerateUUID, 1)
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}

	if e.uuidGenerator == nil {
		return 0, errors.NewOperationNotSupportedError("GenerateUUID")
	}

	uuid, err := e.uuidGenerator.GenerateUUID()
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}
	return uuid, err
}

func (e *TransactionEnv) meterComputation(kind common.ComputationKind, intensity uint) error {
	if e.sth.EnforceComputationLimits {
		return e.sth.State().MeterComputation(kind, intensity)
	}
	return nil
}

func (e *TransactionEnv) MeterComputation(kind common.ComputationKind, intensity uint) error {
	return e.meterComputation(kind, intensity)
}

func (e *TransactionEnv) ComputationUsed() uint64 {
	return uint64(e.sth.State().TotalComputationUsed())
}

func (e *TransactionEnv) SetAccountFrozen(address common.Address, frozen bool) error {

	flowAddress := flow.Address(address)

	if flowAddress == e.ctx.Chain.ServiceAddress() {
		err := errors.NewValueErrorf(flowAddress.String(), "cannot freeze service account")
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	if !e.isAuthorizerServiceAccount() {
		err := errors.NewOperationAuthorizationErrorf("SetAccountFrozen", "accounts can be frozen only by transactions authorized by the service account")
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	err := e.accounts.SetAccountFrozen(flowAddress, frozen)
	if err != nil {
		return fmt.Errorf("setting account frozen failed: %w", err)
	}
	return nil
}

func (e *TransactionEnv) DecodeArgument(b []byte, _ cadence.Type) (cadence.Value, error) {
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

func (e *TransactionEnv) Hash(data []byte, tag string, hashAlgorithm runtime.HashAlgorithm) ([]byte, error) {
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

func (e *TransactionEnv) VerifySignature(
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

func (e *TransactionEnv) ValidatePublicKey(pk *runtime.PublicKey) error {
	err := e.meterComputation(meter.ComputationKindValidatePublicKey, 1)
	if err != nil {
		return fmt.Errorf("validate public key failed: %w", err)
	}

	return crypto.ValidatePublicKey(pk.SignAlgo, pk.PublicKey)
}

// Block Environment Functions

// GetCurrentBlockHeight returns the current block height.
func (e *TransactionEnv) GetCurrentBlockHeight() (uint64, error) {
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
func (e *TransactionEnv) UnsafeRandom() (uint64, error) {
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
func (e *TransactionEnv) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
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

func (e *TransactionEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {

	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvCreateAccount)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindCreateAccount, 1)
	if err != nil {
		return address, err
	}

	e.sth.DisableAllLimitEnforcements() // don't enforce limit during account creation
	defer e.sth.EnableAllLimitEnforcements()

	flowAddress, err := e.addressGenerator.NextAddress()
	if err != nil {
		return address, err
	}

	err = e.accounts.Create(nil, flowAddress)
	if err != nil {
		return address, fmt.Errorf("create account failed: %w", err)
	}

	if e.ctx.ServiceAccountEnabled {
		setupNewAccount := SetupNewAccountInvocation(e, e.traceSpan)
		_, invokeErr := setupNewAccount(flowAddress, payer)

		if invokeErr != nil {
			return address, errors.HandleRuntimeError(invokeErr)
		}
	}

	e.ctx.Metrics.RuntimeSetNumberOfAccounts(e.addressGenerator.AddressCount())
	return runtime.Address(flowAddress), nil
}

// AddEncodedAccountKey adds an encoded public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *TransactionEnv) AddEncodedAccountKey(address runtime.Address, publicKey []byte) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvAddAccountKey)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindAddEncodedAccountKey, 1)
	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}

	// TODO do a call to track the computation usage and memory usage
	e.sth.DisableAllLimitEnforcements() // don't enforce limit during adding a key
	defer e.sth.EnableAllLimitEnforcements()

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}

	err = e.accountKeys.AddEncodedAccountKey(address, publicKey)

	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}
	return nil
}

// RevokeEncodedAccountKey revokes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key revoking fails.
func (e *TransactionEnv) RevokeEncodedAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvRemoveAccountKey)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindRevokeEncodedAccountKey, 1)
	if err != nil {
		return publicKey, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return nil, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	encodedKey, err := e.accountKeys.RemoveAccountKey(address, index)
	if err != nil {
		return nil, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	return encodedKey, nil
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *TransactionEnv) AddAccountKey(
	address runtime.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvAddAccountKey)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindAddAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	accKey, err := e.accountKeys.AddAccountKey(address, publicKey, hashAlgo, weight)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	return accKey, nil
}

// GetAccountKey retrieves a public key by index from an existing account.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key retrieval fails.
func (e *TransactionEnv) GetAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountKey)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindGetAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}

	accKey, err := e.accountKeys.GetAccountKey(address, keyIndex)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}
	return accKey, err
}

// RevokeAccountKey revokes a public key by index from an existing account,
// and returns the revoked key.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key revoking fails.
func (e *TransactionEnv) RevokeAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvRemoveAccountKey)
		defer sp.Finish()
	}

	err := e.meterComputation(meter.ComputationKindRevokeAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("revoke account key failed: %w", err)
	}

	return e.accountKeys.RevokeAccountKey(address, keyIndex)
}

func (e *TransactionEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvUpdateAccountContractCode)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindUpdateAccountContractCode, 1)
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = e.contracts.SetContract(address, name, code, e.getSigningAccounts())
	if err != nil {
		return fmt.Errorf("updating account contract code failed: %w", err)
	}

	return nil
}

func (e *TransactionEnv) GetAccountContractCode(address runtime.Address, name string) (code []byte, err error) {
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

func (e *TransactionEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvRemoveAccountContractCode)
		defer sp.Finish()
	}

	err = e.meterComputation(meter.ComputationKindRemoveAccountContractCode, 1)
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = e.contracts.RemoveContract(address, name, e.getSigningAccounts())
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	return nil
}

func (e *TransactionEnv) GetSigningAccounts() ([]runtime.Address, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetSigningAccounts)
		defer sp.Finish()
	}
	return e.getSigningAccounts(), nil
}

func (e *TransactionEnv) getSigningAccounts() []runtime.Address {
	if e.authorizers == nil {
		e.authorizers = make([]runtime.Address, len(e.tx.Authorizers))

		for i, auth := range e.tx.Authorizers {
			e.authorizers[i] = runtime.Address(auth)
		}
	}
	return e.authorizers
}

func (e *TransactionEnv) ImplementationDebugLog(message string) error {
	e.ctx.Logger.Debug().Msgf("Cadence: %s", message)
	return nil
}

func (e *TransactionEnv) RecordTrace(operation string, location common.Location, duration time.Duration, logs []opentracing.LogRecord) {
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

func (e *TransactionEnv) ProgramParsed(location common.Location, duration time.Duration) {
	e.RecordTrace("parseProgram", location, duration, nil)
	e.metrics.ProgramParsed(location, duration)
}

func (e *TransactionEnv) ProgramChecked(location common.Location, duration time.Duration) {
	e.RecordTrace("checkProgram", location, duration, nil)
	e.metrics.ProgramChecked(location, duration)
}

func (e *TransactionEnv) ProgramInterpreted(location common.Location, duration time.Duration) {
	e.RecordTrace("interpretProgram", location, duration, nil)
	e.metrics.ProgramInterpreted(location, duration)
}

func (e *TransactionEnv) ValueEncoded(duration time.Duration) {
	e.RecordTrace("encodeValue", nil, duration, nil)
	e.metrics.ValueEncoded(duration)
}

func (e *TransactionEnv) ValueDecoded(duration time.Duration) {
	e.RecordTrace("decodeValue", nil, duration, nil)
	e.metrics.ValueDecoded(duration)
}

// Commit commits changes and return a list of updated keys
func (e *TransactionEnv) Commit() ([]programs.ContractUpdateKey, error) {
	// commit changes and return a list of updated keys
	err := e.programs.Cleanup()
	if err != nil {
		return nil, err
	}
	return e.contracts.Commit()
}

func (e *TransactionEnv) BLSVerifyPOP(pk *runtime.PublicKey, sig []byte) (bool, error) {
	return crypto.VerifyPOP(pk, sig)
}

func (e *TransactionEnv) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	return crypto.AggregateSignatures(sigs)
}

func (e *TransactionEnv) BLSAggregatePublicKeys(keys []*runtime.PublicKey) (*runtime.PublicKey, error) {
	return crypto.AggregatePublicKeys(keys)
}

func (e *TransactionEnv) ResourceOwnerChanged(_ *interpreter.CompositeValue, _ common.Address, _ common.Address) {
}
