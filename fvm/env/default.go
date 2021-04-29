package env

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ runtime.Interface = &DefaultEnv{}
var _ runtime.HighLevelStorage = &DefaultEnv{}

type DefaultEnv struct {
	ctx              context.Context
	sth              *state.StateHolder
	vm               context.VirtualMachine
	accounts         *state.Accounts
	contracts        *handler.ContractHandler
	programs         *handler.ProgramsHandler
	accountKeys      *handler.AccountKeyHandler
	metrics          *handler.MetricsHandler
	addressGenerator flow.AddressGenerator
	uuidGenerator    *state.UUIDGenerator
	eventHandler     *handler.EventHandler
	logs             []string
	totalGasUsed     uint64
	rng              *rand.Rand
}

func NewDefaultEnvironment(
	ctx context.Context,
	vm context.VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs) *DefaultEnv {

	accounts := state.NewAccounts(sth)
	generator := state.NewStateBoundAddressGenerator(sth, ctx.Chain)
	contracts := handler.NewContractHandler(accounts,
		ctx.RestrictedDeploymentEnabled,
		[]runtime.Address{runtime.Address(ctx.Chain.ServiceAddress())})

	uuidGenerator := state.NewUUIDGenerator(sth)

	programsHandler := handler.NewProgramsHandler(
		programs, sth,
	)

	eventHandler := handler.NewEventHandler(ctx.Chain,
		ctx.EventCollectionEnabled,
		ctx.ServiceEventCollectionEnabled,
		ctx.EventCollectionByteSizeLimit,
	)

	accountKeys := handler.NewAccountKeyHandler(accounts)

	metrics := handler.NewMetricsHandler(ctx.Metrics)

	env := &DefaultEnv{
		ctx:              ctx,
		sth:              sth,
		vm:               vm,
		metrics:          metrics,
		accounts:         accounts,
		contracts:        contracts,
		accountKeys:      accountKeys,
		addressGenerator: generator,
		uuidGenerator:    uuidGenerator,
		eventHandler:     eventHandler,
		programs:         programsHandler,
	}

	if ctx.BlockHeader != nil {
		env.seedRNG(ctx.BlockHeader)
	}

	return env
}

func (e *DefaultEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	e.rng = rand.New(source)
}

func (e *DefaultEnv) getEvents() []flow.Event {
	return e.eventHandler.Events()
}

func (e *DefaultEnv) getServiceEvents() []flow.Event {
	return e.eventHandler.ServiceEvents()
}

func (e *DefaultEnv) getLogs() []string {
	return e.logs
}

func (e *DefaultEnv) GetValue(owner, key []byte) ([]byte, error) {
	v, err := e.accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (e *DefaultEnv) SetValue(owner, key, value []byte) error {
	err := e.accounts.SetValue(
		flow.BytesToAddress(owner),
		string(key),
		value,
	)
	if err != nil {
		return fmt.Errorf("setting value failed: %w", err)
	}
	return nil
}

func (e *DefaultEnv) ValueExists(owner, key []byte) (exists bool, err error) {
	v, err := e.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("checking value existence failed: %w", err)
	}

	return len(v) > 0, nil
}

func (e *DefaultEnv) GetStorageUsed(address common.Address) (value uint64, err error) {
	value, err = e.accounts.GetStorageUsed(flow.BytesToAddress(address.Bytes()))
	if err != nil {
		return value, fmt.Errorf("getting storage used failed: %w", err)
	}

	return value, nil
}

func (e *DefaultEnv) GetStorageCapacity(address common.Address) (capacity uint64, err error) {
	script := blueprints.StorageCapacityScript(flow.BytesToAddress(address.Bytes()), e.ctx.Chain.ServiceAddress())

	// TODO (ramtin) this shouldn't be this way, it should call the invokeMeta
	// and we handle the errors and still compute the state interactions
	value, err := e.vm.Query(
		e.ctx,
		script,
		e.sth.State().View(),
		e.programs.Programs,
	)
	if err != nil {
		return 0, err
	}

	capacity = value.ToGoValue().(uint64) / 100
	// TODO cleanup
	// var capacity uint64
	// // TODO: Figure out how to handle this error. Currently if a runtime error occurs, storage capacity will be 0.
	// // 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// // 2. There will also be an error in case the accounts balance times megabytesPerFlow constant overflows,
	// //		which shouldn't happen unless the the price of storage is reduced at least 100 fold
	// // 3. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	// if script.Err == nil {
	// 	// Return type is actually a UFix64 with the unit of megabytes so some conversion is necessary
	// 	// divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	// 	capacity =
	// }

	return capacity, nil
}

func (e *DefaultEnv) GetAccountBalance(address common.Address) (balance uint64, err error) {
	script := blueprints.FlowTokenBalanceScript(flow.BytesToAddress(address.Bytes()), e.ctx.Chain.ServiceAddress())

	// TODO similar to the one above
	value, err := e.vm.Query(
		e.ctx,
		script,
		e.sth.State().View(),
		e.programs.Programs,
	)
	if err != nil {
		return 0, err
	}

	balance = value.ToGoValue().(uint64)
	// TODO clean up
	// var balance uint64
	// // TODO: Figure out how to handle this error. Currently if a runtime error occurs, balance will be 0.
	// if script.Err == nil {
	// 	balance = script.Value.ToGoValue().(uint64)
	// }

	return balance, nil
}

func (e *DefaultEnv) GetAccountAvailableBalance(address common.Address) (available uint64, err error) {
	script := blueprints.FlowTokenAvailableBalanceScript(flow.BytesToAddress(address.Bytes()), e.ctx.Chain.ServiceAddress())

	// TODO similar to the one above
	value, err := e.vm.Query(
		e.ctx,
		script,
		e.sth.State().View(),
		e.programs.Programs,
	)
	if err != nil {
		return 0, err
	}

	available = value.ToGoValue().(uint64)
	// var balance uint64
	// // TODO: Figure out how to handle this error. Currently if a runtime error occurs, available balance will be 0.
	// // 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// // 2. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	// if script.Err == nil {
	// 	balance = script.
	// }

	return available, nil
}

func (e *DefaultEnv) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
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

func (e *DefaultEnv) GetCode(location runtime.Location) ([]byte, error) {
	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, errors.NewInvalidLocationErrorf(location, "expecting an AddressLocation, but other location types are passed")
	}

	address := flow.BytesToAddress(contractLocation.Address.Bytes())

	err := e.accounts.CheckAccountNotFrozen(address)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	add, err := e.contracts.GetContract(contractLocation.Address, contractLocation.Name)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	return add, nil
}

func (e *DefaultEnv) GetProgram(location common.Location) (*interpreter.Program, error) {
	if addressLocation, ok := location.(common.AddressLocation); ok {
		address := flow.BytesToAddress(addressLocation.Address.Bytes())

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

func (e *DefaultEnv) SetProgram(location common.Location, program *interpreter.Program) error {
	err := e.programs.Set(location, program)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}
	return nil
}

func (e *DefaultEnv) ProgramLog(message string) error {
	if e.ctx.CadenceLoggingEnabled {
		e.logs = append(e.logs, message)
	}
	return nil
}

func (e *DefaultEnv) EmitEvent(event cadence.Event) error {
	return errors.NewOperationNotSupportedError("EmitEvent")
}

func (e *DefaultEnv) GenerateUUID() (uint64, error) {
	if e.uuidGenerator == nil {
		return 0, errors.NewOperationNotSupportedError("GenerateUUID")
	}

	uuid, err := e.uuidGenerator.GenerateUUID()
	if err != nil {
		return 0, fmt.Errorf("generating uuid failed: %w", err)
	}
	return uuid, err
}

func (e *DefaultEnv) GetComputationLimit() uint64 {
	return e.ctx.GasLimit
}

func (e *DefaultEnv) SetComputationUsed(used uint64) error {
	e.totalGasUsed = used
	return nil
}

func (e *DefaultEnv) GetComputationUsed() uint64 {
	return e.totalGasUsed
}

func (e *DefaultEnv) SetAccountFrozen(address common.Address, frozen bool) error {
	return errors.NewOperationNotSupportedError("SetAccountFrozen")
}

func (e *DefaultEnv) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	v, err := jsoncdc.Decode(b)
	if err != nil {
		err = errors.NewInvalidArgumentErrorf("argument is not json decodable: %w", err)
		return nil, fmt.Errorf("decodeing argument failed: %w", err)
	}

	return v, err
}

func (e *DefaultEnv) Events() []flow.Event {
	return e.eventHandler.Events()
}

func (e *DefaultEnv) Logs() []string {
	return e.logs
}

func (e *DefaultEnv) Hash(data []byte, hashAlgorithm runtime.HashAlgorithm) ([]byte, error) {
	hashAlgo := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	if hashAlgo == crypto.UnknownHashingAlgorithm {
		err := errors.NewValueErrorf(hashAlgorithm.Name(), "hashing algorithm type not found")
		return nil, fmt.Errorf("hashing failed: %w", err)
	}

	hasher := crypto.NewHasher(hashAlgo)
	return hasher.ComputeHash(data), nil
}

func (e *DefaultEnv) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
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
		return false, fmt.Errorf("verifying signature failed: %w", err)
	}

	return valid, nil
}

func (e *DefaultEnv) HighLevelStorageEnabled() bool {
	return e.ctx.SetValueHandler != nil
}

func (e *DefaultEnv) SetCadenceValue(owner common.Address, key string, value cadence.Value) error {
	err := e.ctx.SetValueHandler(flow.Address(owner), key, value)
	if err != nil {
		return fmt.Errorf("setting cadence value failed: %w", err)
	}
	return err
}

// Block Environment Functions

// GetCurrentBlockHeight returns the current block height.
func (e *DefaultEnv) GetCurrentBlockHeight() (uint64, error) {
	if e.ctx.BlockHeader == nil {
		return 0, errors.NewOperationNotSupportedError("GetCurrentBlockHeight")
	}
	return e.ctx.BlockHeader.Height, nil
}

// UnsafeRandom returns a random uint64, where the process of random number derivation is not cryptographically
// secure.
func (e *DefaultEnv) UnsafeRandom() (uint64, error) {
	if e.rng == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	// TODO (ramtin) return errors this assumption that this always succeeds might not be true
	buf := make([]byte, 8)
	_, _ = e.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf), nil
}

// GetBlockAtHeight returns the block at the given height.
func (e *DefaultEnv) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
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
		return runtime.Block{}, false, fmt.Errorf("getting block at height failed for height %v: %w", height, err)
	}

	return runtimeBlockFromHeader(header), true, nil
}

func (e *DefaultEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	return runtime.Address{}, errors.NewOperationNotSupportedError("CreateAccount")
}

func (e *DefaultEnv) AddEncodedAccountKey(address runtime.Address, publicKey []byte) error {
	return errors.NewOperationNotSupportedError("AddEncodedAccountKey")
}

func (e *DefaultEnv) RevokeEncodedAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	return nil, errors.NewOperationNotSupportedError("RevokeEncodedAccountKey")
}

func (e *DefaultEnv) AddAccountKey(
	address runtime.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("AddAccountKey")
}

func (e *DefaultEnv) GetAccountKey(address runtime.Address, index int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("GetAccountKey")
}

func (e *DefaultEnv) RevokeAccountKey(address runtime.Address, index int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("RevokeAccountKey")
}

func (e *DefaultEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	return errors.NewOperationNotSupportedError("UpdateAccountContractCode")
}

func (e *DefaultEnv) GetAccountContractCode(address runtime.Address, name string) (code []byte, err error) {
	code, err = e.GetCode(common.AddressLocation{
		Address: address,
		Name:    name,
	})
	if err != nil {
		return nil, fmt.Errorf("getting account contract code failed: %w", err)
	}

	return code, nil
}

func (e *DefaultEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}

func (e *DefaultEnv) GetSigningAccounts() ([]runtime.Address, error) {
	return nil, errors.NewOperationNotSupportedError("GetSigningAccounts")
}

func (e *DefaultEnv) ImplementationDebugLog(message string) error {
	e.ctx.Logger.Debug().Msgf("Cadence: %s", message)
	return nil
}

func (e *DefaultEnv) ProgramParsed(location common.Location, duration time.Duration) {
	e.metrics.ProgramParsed(location, duration)
}

func (e *DefaultEnv) ProgramChecked(location common.Location, duration time.Duration) {
	e.metrics.ProgramChecked(location, duration)
}

func (e *DefaultEnv) ProgramInterpreted(location common.Location, duration time.Duration) {
	e.metrics.ProgramInterpreted(location, duration)
}

func (e *DefaultEnv) ValueEncoded(duration time.Duration) {
	e.metrics.ValueEncoded(duration)
}

func (e *DefaultEnv) ValueDecoded(duration time.Duration) {
	e.metrics.ValueDecoded(duration)
}

// Commit commits changes and return a list of updated keys
func (e *DefaultEnv) Commit() ([]programs.ContractUpdateKey, error) {
	// commit changes and return a list of updated keys
	err := e.programs.Cleanup()
	if err != nil {
		return nil, err
	}
	return nil, nil
}
