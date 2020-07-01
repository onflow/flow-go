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

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

var _ runtime.Interface = &hostEnv{}

type hostEnv struct {
	vm     *VirtualMachine
	ctx    Context
	ledger Ledger

	runtime.Metrics

	events []cadence.Event
	logs   []string

	transactionEnv *transactionEnv
	rng            *rand.Rand
}

func newEnvironment(vm *VirtualMachine, ctx Context, ledger Ledger) *hostEnv {
	env := &hostEnv{
		vm:      vm,
		ctx:     ctx,
		ledger:  ledger,
		Metrics: &noopMetricsCollector{},
	}

	if ctx.BlockHeader != nil {
		env.seedRNG(ctx.BlockHeader)
	}

	if ctx.Metrics != nil {
		env.Metrics = &metricsCollector{ctx.Metrics}
	}

	return env
}

func (e *hostEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	e.rng = rand.New(source)
}

func (e *hostEnv) setTransaction(tx *flow.TransactionBody) {
	e.transactionEnv = newTransactionEnv(
		e.vm,
		e.ctx,
		e.ledger,
		tx,
	)
}

func (e *hostEnv) getEvents() []cadence.Event {
	return e.events
}

func (e *hostEnv) getLogs() []string {
	return e.logs
}

func (e *hostEnv) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := e.ledger.Get(
		fullKeyHash(
			string(flow.BytesToAddress(owner).Bytes()),
			string(flow.BytesToAddress(controller).Bytes()),
			string(key),
		),
	)
	return v, nil
}

func (e *hostEnv) SetValue(owner, controller, key, value []byte) error {
	e.ledger.Set(
		fullKeyHash(
			string(flow.BytesToAddress(owner).Bytes()),
			string(flow.BytesToAddress(controller).Bytes()),
			string(key),
		),
		value,
	)
	return nil
}

func (e *hostEnv) ValueExists(owner, controller, key []byte) (exists bool, err error) {
	v, err := e.GetValue(owner, controller, key)
	if err != nil {
		return false, err
	}

	return len(v) > 0, nil
}

func (e *hostEnv) ResolveImport(location runtime.Location) ([]byte, error) {
	addressLocation, ok := location.(runtime.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("import location must be an account address")
	}

	address := flow.BytesToAddress(addressLocation)

	code, err := getAccountCode(e.ledger, address)
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, fmt.Errorf("no code deployed at address %s", address)
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
		addressLocation, ok := location.(runtime.AddressLocation)
		if !ok {
			return nil, fmt.Errorf("import location must be an account address")
		}
		key := fullKeyHash(string(addressLocation), string(addressLocation), keyCode)
		e.ledger.Touch(key)
	}

	return program, err
}

func (e *hostEnv) CacheProgram(location ast.Location, program *ast.Program) error {
	if e.ctx.ASTCache == nil {
		return nil
	}

	return e.ctx.ASTCache.SetProgram(location, program)
}

func (e *hostEnv) Log(message string) {
	e.logs = append(e.logs, message)
}

func (e *hostEnv) EmitEvent(event cadence.Event) {
	e.events = append(e.events, event)
}

func (e *hostEnv) GenerateUUID() uint64 {
	// TODO: https://github.com/dapperlabs/flow-go/issues/4141
	return 0
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
	tag []byte,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm string,
	hashAlgorithm string,
) bool {
	panic("implement me")
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

// GetBlockAtHeight returns the block at the given height.
func (e *hostEnv) GetBlockAtHeight(height uint64) (hash runtime.BlockHash, timestamp int64, exists bool, err error) {
	if e.ctx.Blocks == nil {
		panic("GetBlockAtHeight is not supported by this environment")
	}

	block, err := e.ctx.Blocks.ByHeight(height)
	// TODO remove dependency on storage
	if errors.Is(err, storage.ErrNotFound) {
		return runtime.BlockHash{}, 0, false, nil
	} else if err != nil {
		return runtime.BlockHash{}, 0, false, fmt.Errorf(
			"unexpected failure of GetBlockAtHeight, height %v: %w", height, err)
	}

	return runtime.BlockHash(block.ID()), block.Header.Timestamp.UnixNano(), true, nil
}

// Transaction Environment Functions

func (e *hostEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	if e.transactionEnv == nil {
		panic("CreateAccount is not supported by this environment")
	}

	return e.transactionEnv.CreateAccount(payer)
}

func (e *hostEnv) AddAccountKey(address runtime.Address, publicKey []byte) error {
	if e.transactionEnv == nil {
		panic("AddAccountKey is not supported by this environment")
	}

	return e.transactionEnv.AddAccountKey(address, publicKey)
}

func (e *hostEnv) RemoveAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	if e.transactionEnv == nil {
		panic("RemoveAccountKey is not supported by this environment")
	}

	return e.transactionEnv.RemoveAccountKey(address, index)
}

func (e *hostEnv) UpdateAccountCode(address runtime.Address, code []byte) (err error) {
	if e.transactionEnv == nil {
		panic("UpdateAccountCode is not supported by this environment")
	}

	return e.transactionEnv.UpdateAccountCode(address, code)
}

func (e *hostEnv) GetSigningAccounts() []runtime.Address {
	if e.transactionEnv == nil {
		panic("GetSigningAccounts is not supported by this environment")
	}

	return e.transactionEnv.GetSigningAccounts()
}

// Transaction Environment

type transactionEnv struct {
	vm     *VirtualMachine
	ctx    Context
	ledger Ledger

	tx          *flow.TransactionBody
	authorizers []runtime.Address
}

func newTransactionEnv(
	vm *VirtualMachine,
	ctx Context,
	ledger Ledger,
	tx *flow.TransactionBody,
) *transactionEnv {
	return &transactionEnv{
		vm:     vm,
		ctx:    ctx,
		ledger: ledger,
		tx:     tx,
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
	err = e.vm.invokeMetaTransaction(
		e.ctx,
		deductAccountCreationFeeTransaction(
			flow.Address(payer),
			e.vm.chain.ServiceAddress(),
			e.ctx.RestrictedAccountCreationEnabled,
		),
		e.ledger,
	)
	if err != nil {
		return address, err
	}

	var flowAddress flow.Address

	flowAddress, err = createAccount(e.ledger, e.vm.chain, nil)
	if err != nil {
		return address, err
	}

	err = e.vm.invokeMetaTransaction(
		e.ctx,
		initFlowTokenTransaction(flowAddress, e.vm.chain.ServiceAddress()),
		e.ledger,
	)
	if err != nil {
		return address, err
	}

	return runtime.Address(flowAddress), nil
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *transactionEnv) AddAccountKey(address runtime.Address, encPublicKey []byte) (err error) {
	accountAddress := flow.Address(address)

	var ok bool

	ok, err = accountExists(e.ledger, accountAddress)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	var publicKey flow.AccountPublicKey

	publicKey, err = flow.DecodeRuntimeAccountPublicKey(encPublicKey, 0)
	if err != nil {
		return fmt.Errorf("cannot decode runtime public account key: %w", err)
	}

	var publicKeys []flow.AccountPublicKey

	publicKeys, err = getAccountPublicKeys(e.ledger, accountAddress)
	if err != nil {
		return err
	}

	publicKeys = append(publicKeys, publicKey)

	return setAccountPublicKeys(e.ledger, accountAddress, publicKeys)
}

// RemoveAccountKey removes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key deletion fails.
func (e *transactionEnv) RemoveAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	accountAddress := flow.Address(address)

	var ok bool

	ok, err = accountExists(e.ledger, accountAddress)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("account with address %s does not exist", address)
	}

	var publicKeys []flow.AccountPublicKey

	publicKeys, err = getAccountPublicKeys(e.ledger, accountAddress)
	if err != nil {
		return publicKey, err
	}

	if index < 0 || index > len(publicKeys)-1 {
		return publicKey, fmt.Errorf("invalid key index %d, account has %d keys", index, len(publicKeys))
	}

	removedKey := publicKeys[index]

	publicKeys = append(publicKeys[:index], publicKeys[index+1:]...)

	err = setAccountPublicKeys(e.ledger, accountAddress, publicKeys)
	if err != nil {
		return publicKey, err
	}

	var removedKeyBytes []byte

	removedKeyBytes, err = flow.EncodeRuntimeAccountPublicKey(removedKey)
	if err != nil {
		return nil, fmt.Errorf("cannot encode removed runtime account key: %w", err)
	}

	return removedKeyBytes, nil
}

// UpdateAccountCode updates the deployed code on an existing account.
//
// This function returns an error if the specified account does not exist or is
// not a valid signing account.
func (e *transactionEnv) UpdateAccountCode(address runtime.Address, code []byte) (err error) {
	accountAddress := flow.Address(address)

	// currently, every transaction that sets account code (deploys/updates contracts)
	// must be signed by the service account
	if e.ctx.RestrictedDeploymentEnabled && !e.isAuthorizer(runtime.Address(e.vm.chain.ServiceAddress())) {
		return fmt.Errorf("code deployment requires authorization from the service account")
	}

	return setAccountCode(e.ledger, accountAddress, code)
}

func (e *transactionEnv) isAuthorizer(address runtime.Address) bool {
	for _, accountAddress := range e.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
}
