package fvm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go-sdk/crypto"
)

var _ runtime.Interface = &hostEnv{}
var _ runtime.HighLevelStorage = &hostEnv{}

type hostEnv struct {
	ctx              Context
	ledger           state.Ledger
	accounts         *state.Accounts
	addressGenerator flow.AddressGenerator
	uuidGenerator    *UUIDGenerator

	runtime.Metrics

	events []cadence.Event
	logs   []string

	transactionEnv *transactionEnv
	rng            *rand.Rand
}

func (e *hostEnv) Hash(data []byte, hashAlgorithm string) []byte {
	hasher, err := crypto.NewHasher(crypto.StringToHashAlgorithm(hashAlgorithm))
	if err != nil {
		panic(fmt.Errorf("cannot create hasher: %w", err))
	}
	return hasher.ComputeHash(data)
}

func newEnvironment(ctx Context, ledger state.Ledger) (*hostEnv, error) {
	st := state.NewState(ledger, ctx.MaxStateKeySize, ctx.MaxStateValueSize, ctx.MaxStateInteractionSize)
	accounts := state.NewAccounts(st)
	generator, err := state.NewLedgerBoundAddressGenerator(st, ctx.Chain)
	if err != nil {
		return nil, err
	}

	uuids := state.NewUUIDs(st)
	uuidGenerator := NewUUIDGenerator(uuids)

	env := &hostEnv{
		ctx:              ctx,
		ledger:           ledger,
		Metrics:          &noopMetricsCollector{},
		accounts:         accounts,
		addressGenerator: generator,
		uuidGenerator:    uuidGenerator,
	}

	if ctx.BlockHeader != nil {
		env.seedRNG(ctx.BlockHeader)
	}

	if ctx.Metrics != nil {
		env.Metrics = &metricsCollector{ctx.Metrics}
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

func (e *hostEnv) setTransaction(vm *VirtualMachine, tx *flow.TransactionBody) {
	e.transactionEnv = newTransactionEnv(
		vm,
		e.ctx,
		e.ledger,
		e.accounts,
		e.addressGenerator,
		tx,
	)
}

func (e *hostEnv) getEvents() []cadence.Event {
	return e.events
}

func (e *hostEnv) getLogs() []string {
	return e.logs
}

func (e *hostEnv) GetValue(owner, key []byte) ([]byte, error) {
	v, _ := e.ledger.Get(

		string(owner),
		"", // TODO: Remove empty controller key
		string(key),
	)
	return v, nil
}

func (e *hostEnv) SetValue(owner, key, value []byte) error {
	e.ledger.Set(
		string(owner),
		"", // TODO: Remove empty controller key
		string(key),
		value,
	)
	return nil
}

func (e *hostEnv) ValueExists(owner, key []byte) (exists bool, err error) {
	v, err := e.GetValue(owner, key)
	if err != nil {
		return false, err
	}

	return len(v) > 0, nil
}

func (e *hostEnv) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) []runtime.ResolvedLocation {

	addressLocation, isAddress := location.(runtime.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.

	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address

	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)
		contractNames, err := e.accounts.GetContractNames(address)
		if err != nil {
			panic(err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations

		if len(contractNames) == 0 {
			return nil
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
			Location: runtime.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations
}

func (e *hostEnv) GetCode(location runtime.Location) ([]byte, error) {
	contractLocation, ok := location.(runtime.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("can only get code for an account contract (an AddressLocation)")
	}

	address := flow.BytesToAddress(contractLocation.Address.Bytes())

	code, err := e.accounts.GetContract(contractLocation.Name, address)
	if err != nil {
		return nil, err
	}

	return code, nil
}

func (e *hostEnv) GetCachedProgram(location ast.Location) (*ast.Program, error) {
	if e.ctx.ASTCache == nil {
		return nil, nil
	}

	program, err := e.ctx.ASTCache.GetProgram(location)
	if program != nil {
		// Program was found within cache, do an explicit ledger register touch
		// to ensure consistent reads during chunk verification.
		if addressLocation, ok := location.(runtime.AddressLocation); ok {
			e.accounts.TouchContract(addressLocation.Name, flow.BytesToAddress(addressLocation.Address.Bytes()))
		}
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return program, err
}

func (e *hostEnv) CacheProgram(location ast.Location, program *ast.Program) error {
	if e.ctx.ASTCache == nil {
		return nil
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.ctx.ASTCache.SetProgram(location, program)
}

func (e *hostEnv) Log(message string) {
	e.logs = append(e.logs, message)
}

func (e *hostEnv) EmitEvent(event cadence.Event) {
	e.events = append(e.events, event)
}

func (e *hostEnv) GenerateUUID() uint64 {
	uuid, err := e.uuidGenerator.GenerateUUID()
	if err != nil {
		// TODO - Return error once Cadence interface accommodates it
		panic(fmt.Errorf("cannot get UUID: %w", err))
	}

	return uuid
}

func (e *hostEnv) GetComputationLimit() uint64 {
	if e.transactionEnv != nil {
		return e.transactionEnv.GetComputationLimit()
	}

	return e.ctx.GasLimit
}

func (e *hostEnv) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	return jsoncdc.Decode(b)
}

func (e *hostEnv) Events() []cadence.Event {
	return e.events
}

func (e *hostEnv) Logs() []string {
	return e.logs
}

func (e *hostEnv) VerifySignature(
	signature []byte,
	tag string,
	message []byte,
	rawPublicKey []byte,
	rawSigAlgo string,
	rawHashAlgo string,
) bool {
	valid, err := verifySignatureFromRuntime(
		e.ctx.SignatureVerifier,
		signature,
		tag,
		message,
		rawPublicKey,
		rawSigAlgo,
		rawHashAlgo,
	)

	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		panic(err)
	}

	return valid
}

func (e *hostEnv) HighLevelStorageEnabled() bool {
	return e.ctx.SetValueHandler != nil
}

func (e *hostEnv) SetCadenceValue(owner common.Address, key string, value cadence.Value) error {
	return e.ctx.SetValueHandler(flow.Address(owner), key, value)
}

// Block Environment Functions

// GetCurrentBlockHeight returns the current block height.
func (e *hostEnv) GetCurrentBlockHeight() uint64 {
	if e.ctx.BlockHeader == nil {
		panic("GetCurrentBlockHeight is not supported by this environment")
	}

	return e.ctx.BlockHeader.Height
}

// UnsafeRandom returns a random uint64, where the process of random number derivation is not cryptographically
// secure.
func (e *hostEnv) UnsafeRandom() uint64 {
	if e.rng == nil {
		panic("UnsafeRandom is not supported by this environment")
	}

	buf := make([]byte, 8)
	_, _ = e.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf)
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
	if e.ctx.Blocks == nil {
		panic("GetBlockAtHeight is not supported by this environment")
	}

	if e.ctx.BlockHeader != nil && height == e.ctx.BlockHeader.Height {
		return runtimeBlockFromHeader(e.ctx.BlockHeader), true, nil
	}

	block, err := e.ctx.Blocks.ByHeight(height)
	// TODO: remove dependency on storage
	if errors.Is(err, storage.ErrNotFound) {
		return runtime.Block{}, false, nil
	} else if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return runtime.Block{}, false, fmt.Errorf("unexpected failure of GetBlockAtHeight, height %v: %w", height, err)
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return runtimeBlockFromHeader(block.Header), true, nil
}

// Transaction Environment Functions

func (e *hostEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	if e.transactionEnv == nil {
		panic("CreateAccount is not supported by this environment")
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.CreateAccount(payer)
}

func (e *hostEnv) AddAccountKey(address runtime.Address, publicKey []byte) error {
	if e.transactionEnv == nil {
		panic("AddAccountKey is not supported by this environment")
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.AddAccountKey(address, publicKey)
}

func (e *hostEnv) RemoveAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	if e.transactionEnv == nil {
		panic("RemoveAccountKey is not supported by this environment")
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.RemoveAccountKey(address, index)
}

func (e *hostEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	if e.transactionEnv == nil {
		panic("UpdateAccountContractCode is not supported by this environment")
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.UpdateAccountContractCode(address, name, code)
}

func (e *hostEnv) GetAccountContractCode(address runtime.Address, name string) (code []byte, err error) {
	return e.GetCode(runtime.AddressLocation{
		Address: address,
		Name:    name,
	})
}

func (e *hostEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	if e.transactionEnv == nil {
		panic("RemoveAccountContractCode is not supported by this environment")
	}

	// TODO: improve error passing https://github.com/onflow/cadence/issues/202
	return e.transactionEnv.RemoveAccountContractCode(address, name)
}

func (e *transactionEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	accountAddress := flow.Address(address)

	// must be signed by the service account
	if e.ctx.RestrictedDeploymentEnabled && !e.isAuthorizer(runtime.Address(e.ctx.Chain.ServiceAddress())) {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return fmt.Errorf("code deployment requires authorization from the service account")
	}

	return e.accounts.SetContract(name, accountAddress, code)
}

func (e *transactionEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	accountAddress := flow.Address(address)

	// must be signed by the service account
	if e.ctx.RestrictedDeploymentEnabled && !e.isAuthorizer(runtime.Address(e.ctx.Chain.ServiceAddress())) {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return fmt.Errorf("code deployment requires authorization from the service account")
	}

	return e.accounts.DeleteContract(name, accountAddress)
}

func (e *hostEnv) GetSigningAccounts() []runtime.Address {
	if e.transactionEnv == nil {
		panic("GetSigningAccounts is not supported by this environment")
	}

	return e.transactionEnv.GetSigningAccounts()
}

// Transaction Environment

type transactionEnv struct {
	vm               *VirtualMachine
	ctx              Context
	ledger           state.Ledger
	accounts         *state.Accounts
	addressGenerator flow.AddressGenerator

	tx          *flow.TransactionBody
	authorizers []runtime.Address
}

func newTransactionEnv(
	vm *VirtualMachine,
	ctx Context,
	ledger state.Ledger,
	accounts *state.Accounts,
	addressGenerator flow.AddressGenerator,
	tx *flow.TransactionBody,
) *transactionEnv {
	return &transactionEnv{
		vm:               vm,
		ctx:              ctx,
		ledger:           ledger,
		accounts:         accounts,
		addressGenerator: addressGenerator,
		tx:               tx,
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

func (e *transactionEnv) GetComputationLimit() uint64 {
	return e.tx.GasLimit
}

func (e *transactionEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	if e.ctx.ServiceAccountEnabled {
		err = e.vm.invokeMetaTransaction(
			e.ctx,
			deductAccountCreationFeeTransaction(
				flow.Address(payer),
				e.ctx.Chain.ServiceAddress(),
				e.ctx.RestrictedAccountCreationEnabled,
			),
			e.ledger,
		)
		if err != nil {
			// TODO: improve error passing https://github.com/onflow/cadence/issues/202
			return address, err
		}
	}

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
			initFlowTokenTransaction(flowAddress, e.ctx.Chain.ServiceAddress()),
			e.ledger,
		)
		if err != nil {
			// TODO: improve error passing https://github.com/onflow/cadence/issues/202
			return address, err
		}
	}

	return runtime.Address(flowAddress), nil
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *transactionEnv) AddAccountKey(address runtime.Address, encodedPublicKey []byte) (err error) {
	accountAddress := flow.Address(address)

	var ok bool

	ok, err = e.accounts.Exists(accountAddress)
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
		return fmt.Errorf("cannot decode runtime public account key: %w", err)
	}

	err = e.accounts.AppendPublicKey(accountAddress, publicKey)
	if err != nil {
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

	var ok bool

	ok, err = e.accounts.Exists(accountAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, err
	}

	if !ok {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return nil, fmt.Errorf("account with address %s does not exist", address)
	}

	if keyIndex < 0 {
		return nil, fmt.Errorf("key index must be positve, received %d", keyIndex)
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = e.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
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

func (e *transactionEnv) isAuthorizer(address runtime.Address) bool {
	for _, accountAddress := range e.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}
	return false
}
