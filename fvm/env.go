package fvm

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"

	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	fvmEvent "github.com/onflow/flow-go/fvm/event"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go-sdk/crypto"
)

var _ runtime.Interface = &hostEnv{}
var _ runtime.HighLevelStorage = &hostEnv{}

type hostEnv struct {
	ctx                Context
	sth                *state.StateHolder
	vm                 *VirtualMachine
	accounts           *state.Accounts
	contracts          *handler.ContractHandler
	programs           *handler.ProgramsHandler
	addressGenerator   flow.AddressGenerator
	uuidGenerator      *state.UUIDGenerator
	metrics            runtime.Metrics
	events             []flow.Event
	serviceEvents      []flow.Event
	totalEventByteSize uint64
	logs               []string
	totalGasUsed       uint64
	transactionEnv     *transactionEnv
	rng                *rand.Rand
}

func newEnvironment(ctx Context, vm *VirtualMachine, sth *state.StateHolder, programs *programs.Programs) (*hostEnv, error) {
	accounts := state.NewAccounts(sth)
	generator, err := state.NewStateBoundAddressGenerator(sth, ctx.Chain)
	if err != nil {
		return nil, err
	}

	contracts := handler.NewContractHandler(accounts,
		ctx.RestrictedDeploymentEnabled,
		[]runtime.Address{runtime.Address(ctx.Chain.ServiceAddress())})

	uuidGenerator := state.NewUUIDGenerator(sth)

	programsHandler := handler.NewProgramsHandler(
		programs, sth,
	)

	env := &hostEnv{
		ctx:                ctx,
		sth:                sth,
		vm:                 vm,
		metrics:            &noopMetricsCollector{},
		accounts:           accounts,
		contracts:          contracts,
		addressGenerator:   generator,
		uuidGenerator:      uuidGenerator,
		totalEventByteSize: uint64(0),
		programs:           programsHandler,
	}

	if ctx.BlockHeader != nil {
		env.seedRNG(ctx.BlockHeader)
	}

	if ctx.Metrics != nil {
		env.metrics = &metricsCollector{ctx.Metrics}
	}

	return env, nil
}

func (e *hostEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	e.rng = rand.New(source)
}

func (e *hostEnv) setTransaction(tx *flow.TransactionBody, txIndex uint32) {
	e.transactionEnv = newTransactionEnv(
		e.vm,
		e.ctx,
		e.sth,
		e.programs,
		e.accounts,
		e.contracts,
		e.addressGenerator,
		tx,
		txIndex,
	)
}

func (e *hostEnv) setTraceSpan(span opentracing.Span) {
	e.transactionEnv.traceSpan = span
}

func (e *hostEnv) getEvents() []flow.Event {
	return e.events
}

func (e *hostEnv) getServiceEvents() []flow.Event {
	return e.serviceEvents
}

func (e *hostEnv) getLogs() []string {
	return e.logs
}

func (e *hostEnv) isTraceable() bool {
	return e.ctx.Tracer != nil && e.transactionEnv != nil && e.transactionEnv.traceSpan != nil
}

func (e *hostEnv) GetValue(owner, key []byte) ([]byte, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetValue)
		sp.LogFields(
			traceLog.String("owner", hex.EncodeToString(owner)),
			traceLog.String("key", string(key)),
		)
		defer sp.Finish()
	}

	v, _ := e.accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	return v, nil
}

func (e *hostEnv) SetValue(owner, key, value []byte) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvSetValue)
		sp.LogFields(
			traceLog.String("owner", hex.EncodeToString(owner)),
			traceLog.String("key", string(key)),
		)
		defer sp.Finish()
	}

	return e.accounts.SetValue(
		flow.BytesToAddress(owner),
		string(key),
		value,
	)
}

func (e *hostEnv) ValueExists(owner, key []byte) (exists bool, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvValueExists)
		defer sp.Finish()
	}

	v, err := e.GetValue(owner, key)
	if err != nil {
		return false, err
	}

	return len(v) > 0, nil
}

func (e *hostEnv) GetStorageUsed(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetStorageUsed)
		defer sp.Finish()
	}

	return e.accounts.GetStorageUsed(flow.BytesToAddress(address.Bytes()))
}

func (e *hostEnv) GetStorageCapacity(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetStorageCapacity)
		defer sp.Finish()
	}

	script := getStorageCapacityScript(flow.BytesToAddress(address.Bytes()), e.ctx.Chain.ServiceAddress())

	err = e.vm.Run(
		e.ctx,
		script,
		e.sth.State().View(),
		e.programs.Programs,
	)
	if err != nil {
		return 0, err
	}

	var capacity uint64
	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, storage capacity will be 0.
	// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// 2. There will also be an error in case the accounts balance times megabytesPerFlow constant overflows,
	//		which shouldn't happen unless the the price of storage is reduced at least 100 fold
	// 3. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	if script.Err == nil {
		// Return type is actually a UFix64 with the unit of megabytes so some conversion is necessary
		// divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega)) to get bytes (rounded down)
		capacity = script.Value.ToGoValue().(uint64) / 100
	}

	return capacity, nil
}

func (e *hostEnv) GetAccountBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.Finish()
	}

	script := getFlowTokenBalanceScript(flow.BytesToAddress(address.Bytes()), e.ctx.Chain.ServiceAddress())

	err = e.vm.Run(
		e.ctx,
		script,
		e.sth.State().View(),
		e.programs.Programs,
	)
	if err != nil {
		return 0, err
	}

	var balance uint64
	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, balance will be 0.
	// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// 2. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	if script.Err == nil {
		balance = script.Value.ToGoValue().(uint64)
	}

	return balance, nil
}

func (e *hostEnv) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvResolveLocation)
		defer sp.Finish()
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
			return nil, fmt.Errorf("resolve location: %w", err)
		}

		contractNames, err := e.contracts.GetContractNames(addressLocation.Address)
		if err != nil {
			panic(err)
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

func (e *hostEnv) GetCode(location runtime.Location) ([]byte, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetCode)
		defer sp.Finish()
	}

	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("can only get code for an account contract (an AddressLocation)")
	}

	address := flow.BytesToAddress(contractLocation.Address.Bytes())

	err := e.accounts.CheckAccountNotFrozen(address)
	if err != nil {
		return nil, fmt.Errorf("get code: %w", err)
	}

	return e.contracts.GetContract(contractLocation.Address, contractLocation.Name)
}

func (e *hostEnv) GetProgram(location common.Location) (*interpreter.Program, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetProgram)
		defer sp.Finish()
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		address := flow.BytesToAddress(addressLocation.Address.Bytes())

		freezeError := e.accounts.CheckAccountNotFrozen(address)

		if freezeError != nil {
			return nil, freezeError
		}
	}

	program, has := e.programs.Get(location)
	if has {
		return program, nil
	}

	return nil, nil
}

func (e *hostEnv) SetProgram(location common.Location, program *interpreter.Program) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvSetProgram)
		defer sp.Finish()
	}

	return e.programs.Set(location, program)
}

func (e *hostEnv) ProgramLog(message string) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvProgramLog)
		defer sp.Finish()
	}

	if e.ctx.CadenceLoggingEnabled {
		e.logs = append(e.logs, message)
	}
	return nil
}

func (e *hostEnv) EmitEvent(event cadence.Event) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvEmitEvent)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return errors.New("emitting events is not supported")
	}

	payload, err := jsoncdc.Encode(event)
	if err != nil {
		return fmt.Errorf("failed to json encode a cadence event: %w", err)
	}

	e.totalEventByteSize += uint64(len(payload))

	// skip limit if payer is service account
	if e.transactionEnv.tx.Payer != e.ctx.Chain.ServiceAddress() {
		if e.totalEventByteSize > e.ctx.EventCollectionByteSizeLimit {
			return &fvmErrors.EventLimitExceededError{
				TotalByteSize: e.totalEventByteSize,
				Limit:         e.ctx.EventCollectionByteSizeLimit,
			}
		}
	}

	flowEvent := flow.Event{
		Type:             flow.EventType(event.EventType.ID()),
		TransactionID:    e.transactionEnv.TxID(),
		TransactionIndex: e.transactionEnv.TxIndex(),
		EventIndex:       uint32(len(e.events)),
		Payload:          payload,
	}

	if fvmEvent.IsServiceEvent(event, e.ctx.Chain) {
		e.serviceEvents = append(e.serviceEvents, flowEvent)
	}

	e.events = append(e.events, flowEvent)
	return nil
}

func (e *hostEnv) GenerateUUID() (uint64, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGenerateUUID)
		defer sp.Finish()
	}

	// TODO add not supported
	uuid, err := e.uuidGenerator.GenerateUUID()
	return uuid, err
}

func (e *hostEnv) GetComputationLimit() uint64 {
	if e.transactionEnv != nil {
		return e.transactionEnv.GetComputationLimit()
	}

	return e.ctx.GasLimit
}

func (e *hostEnv) SetComputationUsed(used uint64) error {
	e.totalGasUsed = used
	return nil
}

func (e *hostEnv) SetAccountFrozen(address common.Address, frozen bool) error {

	flowAddress := flow.Address(address)

	if flowAddress == e.ctx.Chain.ServiceAddress() {
		return fmt.Errorf("cannot freeze service account")
	}

	if !e.transactionEnv.isAuthorizerServiceAccount() {
		return fmt.Errorf("SetAccountFrozen can only be used in transactions authorized by service account")
	}

	err := e.accounts.SetAccountFrozen(flowAddress, frozen)
	if err != nil {
		return fmt.Errorf("cannot set account [%s] frozen [%t]: %w", flowAddress, frozen, err)
	}
	return nil
}

func (e *hostEnv) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvDecodeArgument)
		defer sp.Finish()
	}

	return jsoncdc.Decode(b)
}

func (e *hostEnv) Events() []flow.Event {
	return e.events
}

func (e *hostEnv) Logs() []string {
	return e.logs
}

func (e *hostEnv) Hash(data []byte, hashAlgorithm runtime.HashAlgorithm) ([]byte, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvHash)
		defer sp.Finish()
	}

	hashAlgo := RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	if hashAlgo == crypto.UnknownHashAlgorithm {
		panic(fmt.Errorf("unknown hash algorithm: %s", hashAlgorithm))
	}

	hasher, err := crypto.NewHasher(hashAlgo)
	if err != nil {
		panic(fmt.Errorf("cannot create hasher: %w", err))
	}

	return hasher.ComputeHash(data), nil
}

func (e *hostEnv) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvVerifySignature)
		defer sp.Finish()
	}

	valid, err := verifySignatureFromRuntime(
		e.ctx.SignatureVerifier,
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm,
	)

	if err != nil {
		return false, err
	}

	return valid, nil
}

func (e *hostEnv) HighLevelStorageEnabled() bool {
	return e.ctx.SetValueHandler != nil
}

func (e *hostEnv) SetCadenceValue(owner common.Address, key string, value cadence.Value) error {
	return e.ctx.SetValueHandler(flow.Address(owner), key, value)
}

// Block Environment Functions

// GetCurrentBlockHeight returns the current block height.
func (e *hostEnv) GetCurrentBlockHeight() (uint64, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetCurrentBlockHeight)
		defer sp.Finish()
	}

	if e.ctx.BlockHeader == nil {
		return 0, errors.New("getting the current block height is not supported")
	}
	return e.ctx.BlockHeader.Height, nil
}

// UnsafeRandom returns a random uint64, where the process of random number derivation is not cryptographically
// secure.
func (e *hostEnv) UnsafeRandom() (uint64, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvUnsafeRandom)
		defer sp.Finish()
	}

	if e.rng == nil {
		return 0, errors.New("unsafe random is not supported")
	}
	buf := make([]byte, 8)
	_, _ = e.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf), nil
}

func runtimeBlockFromHeader(header *flow.Header) runtime.Block {
	return runtime.Block{
		Height:    header.Height,
		View:      header.View,
		Hash:      runtime.BlockHash(header.ID()),
		Timestamp: header.Timestamp.UnixNano(),
	}
}

// GetBlockAtHeight returns the block at the given height.
func (e *hostEnv) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetBlockAtHeight)
		defer sp.Finish()
	}

	if e.ctx.Blocks == nil {
		return runtime.Block{}, false, errors.New("getting block information is not supported")
	}

	if e.ctx.BlockHeader != nil && height == e.ctx.BlockHeader.Height {
		return runtimeBlockFromHeader(e.ctx.BlockHeader), true, nil
	}

	header, err := e.ctx.Blocks.ByHeightFrom(height, e.ctx.BlockHeader)
	// TODO: remove dependency on storage
	if errors.Is(err, storage.ErrNotFound) {
		return runtime.Block{}, false, nil
	} else if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return runtime.Block{}, false, fmt.Errorf("unexpected failure of GetBlockAtHeight, height %v: %w", height, err)
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return runtimeBlockFromHeader(header), true, nil
}

// Transaction Environment Functions

func (e *hostEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvCreateAccount)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return runtime.Address{}, errors.New("creating accounts is not supported")
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.CreateAccount(payer)
}

func (e *hostEnv) AddEncodedAccountKey(address runtime.Address, publicKey []byte) error {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvAddAccountKey)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return errors.New("adding account keys is not supported")
	}

	err := e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("add count key: %w", err)
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.AddEncodedAccountKey(address, publicKey)
}

func (e *hostEnv) RevokeEncodedAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvRemoveAccountKey)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return nil, errors.New("removing account keys is not supported")
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return nil, fmt.Errorf("remove account key: %w", err)
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.RemoveAccountKey(address, index)
}

func (e *hostEnv) AddAccountKey(
	address runtime.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (*runtime.AccountKey, error) {

	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvAddAccountKey)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return nil, errors.New("adding account keys is not supported")
	}

	return e.transactionEnv.AddAccountKey(address, publicKey, hashAlgo, weight)
}

func (e *hostEnv) GetAccountKey(address runtime.Address, index int) (*runtime.AccountKey, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetAccountKey)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return nil, errors.New("getting account keys is not supported")
	}

	return e.transactionEnv.GetAccountKey(address, index)
}

func (e *hostEnv) RevokeAccountKey(address runtime.Address, index int) (*runtime.AccountKey, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvRemoveAccountKey)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return nil, errors.New("revoking account keys is not supported")
	}

	return e.transactionEnv.RevokeAccountKey(address, index)
}

func (e *hostEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvUpdateAccountContractCode)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return errors.New("updating account contract code is not supported")
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("update account contract code: %w", err)
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.UpdateAccountContractCode(address, name, code)
}

func (e *hostEnv) GetAccountContractCode(address runtime.Address, name string) (code []byte, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetAccountContractCode)
		defer sp.Finish()
	}

	return e.GetCode(common.AddressLocation{
		Address: address,
		Name:    name,
	})
}

func (e *hostEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvRemoveAccountContractCode)
		defer sp.Finish()
	}

	if e.transactionEnv == nil {
		return errors.New("removing account contracts is not supported")
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("remove account contract code: %w", err)
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.RemoveAccountContractCode(address, name)
}

func (e *hostEnv) GetSigningAccounts() ([]runtime.Address, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.transactionEnv.traceSpan, trace.FVMEnvGetSigningAccounts)
		defer sp.Finish()
	}
	if e.transactionEnv == nil {
		return nil, errors.New("getting signer accounts is not supported")
	}

	return e.transactionEnv.GetSigningAccounts(), nil
}

func (e *hostEnv) ImplementationDebugLog(message string) error {
	e.ctx.Logger.Debug().Msgf("Cadence: %s", message)
	return nil
}

func (e *hostEnv) ProgramParsed(location common.Location, duration time.Duration) {
	if e.isTraceable() {
		e.ctx.Tracer.RecordSpanFromParent(e.transactionEnv.traceSpan, trace.FVMCadenceParseProgram, duration,
			[]opentracing.LogRecord{{Timestamp: time.Now(),
				Fields: []traceLog.Field{traceLog.String("location", location.String())},
			},
			},
		)
	}
	e.metrics.ProgramParsed(location, duration)
}

func (e *hostEnv) ProgramChecked(location common.Location, duration time.Duration) {
	if e.isTraceable() {
		e.ctx.Tracer.RecordSpanFromParent(e.transactionEnv.traceSpan, trace.FVMCadenceCheckProgram, duration,
			[]opentracing.LogRecord{{Timestamp: time.Now(),
				Fields: []traceLog.Field{traceLog.String("location", location.String())},
			},
			},
		)
	}
	e.metrics.ProgramChecked(location, duration)
}

func (e *hostEnv) ProgramInterpreted(location common.Location, duration time.Duration) {
	if e.isTraceable() {
		e.ctx.Tracer.RecordSpanFromParent(e.transactionEnv.traceSpan, trace.FVMCadenceInterpretProgram, duration,
			[]opentracing.LogRecord{{Timestamp: time.Now(),
				Fields: []traceLog.Field{traceLog.String("location", location.String())},
			},
			},
		)
	}
	e.metrics.ProgramInterpreted(location, duration)
}

func (e *hostEnv) ValueEncoded(duration time.Duration) {
	if e.isTraceable() {
		e.ctx.Tracer.RecordSpanFromParent(e.transactionEnv.traceSpan, trace.FVMCadenceEncodeValue, duration,
			[]opentracing.LogRecord{},
		)
	}
	e.metrics.ValueEncoded(duration)
}

func (e *hostEnv) ValueDecoded(duration time.Duration) {
	if e.isTraceable() {
		e.ctx.Tracer.RecordSpanFromParent(e.transactionEnv.traceSpan, trace.FVMCadenceDecodeValue, duration,
			[]opentracing.LogRecord{},
		)
	}
	e.metrics.ValueDecoded(duration)
}

func (e *hostEnv) Commit() ([]programs.ContractUpdateKey, error) {
	// commit changes and return a list of updated keys
	err := e.programs.Cleanup()
	if err != nil {
		return nil, err
	}
	return e.contracts.Commit()
}

// Transaction Environment
type transactionEnv struct {
	vm               *VirtualMachine
	ctx              Context
	sth              *state.StateHolder
	programs         *handler.ProgramsHandler
	accounts         *state.Accounts
	contracts        *handler.ContractHandler
	addressGenerator flow.AddressGenerator
	tx               *flow.TransactionBody
	txIndex          uint32
	txID             flow.Identifier
	traceSpan        opentracing.Span
	authorizers      []runtime.Address
}

func newTransactionEnv(
	vm *VirtualMachine,
	ctx Context,
	sth *state.StateHolder,
	programs *handler.ProgramsHandler,
	accounts *state.Accounts,
	contracts *handler.ContractHandler,
	addressGenerator flow.AddressGenerator,
	tx *flow.TransactionBody,
	txIndex uint32,
) *transactionEnv {
	return &transactionEnv{
		vm:               vm,
		ctx:              ctx,
		sth:              sth,
		programs:         programs,
		accounts:         accounts,
		contracts:        contracts,
		addressGenerator: addressGenerator,
		tx:               tx,
		txIndex:          txIndex,
		txID:             tx.ID(),
	}
}

func (e *transactionEnv) GetSigningAccounts() []runtime.Address {
	if e.authorizers == nil {
		e.authorizers = make([]runtime.Address, len(e.tx.Authorizers))

		for i, auth := range e.tx.Authorizers {
			e.authorizers[i] = runtime.Address(auth)
		}
	}

	return e.authorizers
}

func (e *transactionEnv) TxIndex() uint32 {
	return e.txIndex
}

func (e *transactionEnv) TxID() flow.Identifier {
	return e.txID
}

func (e *transactionEnv) GetComputationLimit() uint64 {
	return e.tx.GasLimit
}

func (e *transactionEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	flowAddress, err := e.addressGenerator.NextAddress()
	if err != nil {
		return address, err
	}

	err = e.accounts.Create(nil, flowAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return address, err
	}

	if e.ctx.ServiceAccountEnabled {
		err = e.vm.invokeMetaTransaction(
			e.ctx,
			initAccountTransaction(
				flow.Address(payer),
				flowAddress,
				e.ctx.Chain.ServiceAddress(),
				e.ctx.RestrictedAccountCreationEnabled),
			e.sth,
			e.programs.Programs,
		)
		if err != nil {
			// TODO: improve error passing https://github.com/onflow/cadence/issues/202
			return address, err
		}
	}

	return runtime.Address(flowAddress), nil
}

// AddEncodedAccountKey adds an encoded public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *transactionEnv) AddEncodedAccountKey(address runtime.Address, encodedPublicKey []byte) (err error) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return err
	}

	if !ok {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return fmt.Errorf("account with address %s does not exist", address)
	}

	var publicKey flow.AccountPublicKey

	publicKey, err = flow.DecodeRuntimeAccountPublicKey(encodedPublicKey, 0)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return fmt.Errorf("cannot decode runtime public account key: %w", err)
	}

	err = e.accounts.AppendPublicKey(accountAddress, publicKey)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return fmt.Errorf("failed to add public key to account: %w", err)
	}

	return nil
}

// RemoveAccountKey revokes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key revoking fails.
func (e *transactionEnv) RemoveAccountKey(address runtime.Address, keyIndex int) (encodedPublicKey []byte, err error) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	if !ok {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("account with address %s does not exist", address)
	}

	if keyIndex < 0 {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("key index must be positive, received %d", keyIndex)
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = e.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	// mark this key as revoked
	publicKey.Revoked = true

	encodedPublicKey, err = e.accounts.SetPublicKey(accountAddress, uint64(keyIndex), publicKey)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202 {
		return nil, fmt.Errorf("failed to revoke account key: %w", err)
	}

	return encodedPublicKey, nil
}

func (e *transactionEnv) isAuthorizerServiceAccount() bool {
	return e.isAuthorizer(runtime.Address(e.ctx.Chain.ServiceAddress()))
}

func (e *transactionEnv) isAuthorizer(address runtime.Address) bool {
	for _, accountAddress := range e.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}
	return false
}

func (e *transactionEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	return e.contracts.SetContract(address, name, code, e.GetSigningAccounts())
}

func (e *transactionEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	return e.contracts.RemoveContract(address, name, e.GetSigningAccounts())
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *transactionEnv) AddAccountKey(address runtime.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	if !ok {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("account with address %s does not exist", address)
	}

	signAlgorithm := RuntimeToCryptoSigningAlgorithm(publicKey.SignAlgo)
	if signAlgorithm == crypto.UnknownSignatureAlgorithm {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("cannot decode public key: invalid signature algorithm: %s", publicKey.SignAlgo)
	}

	hashAlgorithm := RuntimeToCryptoHashingAlgorithm(hashAlgo)
	if hashAlgorithm == crypto.UnknownHashAlgorithm {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("cannot decode public key: invalid hash algorithm: %s", hashAlgo)
	}

	decodedPublicKey, err := crypto.DecodePublicKey(signAlgorithm, publicKey.PublicKey)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("cannot decode public key: %w", err)
	}

	keyIndex, err := e.accounts.GetPublicKeyCount(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	accountPublicKey := flow.AccountPublicKey{
		Index:     int(keyIndex),
		PublicKey: decodedPublicKey,
		SignAlgo:  signAlgorithm,
		HashAlgo:  hashAlgorithm,
		SeqNumber: 0,
		Weight:    weight,
		Revoked:   false,
	}

	err = e.accounts.AppendPublicKey(accountAddress, accountPublicKey)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("failed to add public key to account: %w", err)
	}

	return &runtime.AccountKey{
		KeyIndex:  accountPublicKey.Index,
		PublicKey: publicKey,
		HashAlgo:  hashAlgo,
		Weight:    accountPublicKey.Weight,
		IsRevoked: accountPublicKey.Revoked,
	}, nil
}

// RevokeAccountKey revokes a public key by index from an existing account,
// and returns the revoked key.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key revoking fails.
func (e *transactionEnv) RevokeAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	if !ok {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("account with address %s does not exist", address)
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = e.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with no errors.
		// This is to be inline with the Cadence runtime. Otherwise Cadence runtime cannot
		// distinguish between a 'key not found error' vs other internal errors.

		if errors.Is(err, state.ErrAccountPublicKeyNotFound) {
			return nil, nil
		}

		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	// mark this key as revoked
	publicKey.Revoked = true

	_, err = e.accounts.SetPublicKey(accountAddress, uint64(keyIndex), publicKey)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("failed to revoke account key: %w", err)
	}

	// Prepare the account key to return

	signAlgo := CryptoToRuntimeSigningAlgorithm(publicKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf(
			"failed to revoke account key: failed to convert signature algorithm: %s",
			publicKey.SignAlgo,
		)
	}

	hashAlgo := CryptoToRuntimeHashingAlgorithm(publicKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf(
			"failed to revoke account key: failed to convert hash algorithm: %s",
			publicKey.HashAlgo,
		)
	}

	return &runtime.AccountKey{
		KeyIndex: publicKey.Index,
		PublicKey: &runtime.PublicKey{
			PublicKey: publicKey.PublicKey.Encode(),
			SignAlgo:  signAlgo,
		},
		HashAlgo:  hashAlgo,
		Weight:    publicKey.Weight,
		IsRevoked: publicKey.Revoked,
	}, nil
}

// GetAccountKey retrieves a public key by index from an existing account.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key retrieval fails.
func (e *transactionEnv) GetAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	if !ok {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("account with address %s does not exist", address)
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = e.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with no errors.
		// This is to be inline with the Cadence runtime. Otherwise Cadence runtime cannot
		// distinguish between a 'key not found error' vs other internal errors.
		if errors.Is(err, state.ErrAccountPublicKeyNotFound) {
			return nil, nil
		}

		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	// Prepare the account key to return

	signAlgo := CryptoToRuntimeSigningAlgorithm(publicKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf(
			"failed to get account key: failed to convert signature algorithm: %s",
			publicKey.SignAlgo,
		)
	}

	hashAlgo := CryptoToRuntimeHashingAlgorithm(publicKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf(
			"failed to get account key: failed to convert hash algorithm: %s",
			publicKey.HashAlgo,
		)
	}

	return &runtime.AccountKey{
		KeyIndex: publicKey.Index,
		PublicKey: &runtime.PublicKey{
			PublicKey: publicKey.PublicKey.Encode(),
			SignAlgo:  signAlgo,
		},
		HashAlgo:  hashAlgo,
		Weight:    publicKey.Weight,
		IsRevoked: publicKey.Revoked,
	}, nil
}
