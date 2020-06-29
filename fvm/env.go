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
	chain    flow.Chain
	ledger   Ledger
	astCache ASTCache
	blocks   Blocks

	runtime.Metrics

	gasLimit    uint64
	uuid        uint64
	blockHeader *flow.Header
	rng         *rand.Rand

	events []cadence.Event
	logs   []string

	transactionEnv             *transactionEnv
	restrictContractDeployment bool
	restrictAccountCreation    bool
}

func newEnvironment(ledger Ledger, opts Options) *hostEnv {
	env := &hostEnv{
		ledger:                     ledger,
		chain:                      opts.chain,
		astCache:                   opts.astCache,
		blocks:                     opts.blocks,
		Metrics:                    &noopMetricsCollector{},
		gasLimit:                   opts.gasLimit,
		restrictContractDeployment: opts.restrictedDeploymentEnabled,
		restrictAccountCreation:    opts.restrictedAccountCreationEnabled,
	}

	if opts.blockHeader != nil {
		env.setBlockHeader(opts.blockHeader)
		env.seedRNG(opts.blockHeader)
	}

	if opts.metrics != nil {
		env.Metrics = &metricsCollector{opts.metrics}
	}

	return env
}

func (e *hostEnv) setBlockHeader(header *flow.Header) {
	e.blockHeader = header
}

func (e *hostEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block header ID. The random number generator will
	// be used by the UnsafeRandom function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	e.rng = rand.New(source)
}

func (e *hostEnv) setTransaction(
	tx *flow.TransactionBody,
	txCtx Context,
) *hostEnv {
	e.transactionEnv = newTransactionEnv(
		e.ledger,
		e.chain,
		tx,
		txCtx,
		e.restrictContractDeployment,
		e.restrictAccountCreation,
	)
	return e
}

func (e *hostEnv) getEvents() []cadence.Event {
	return e.events
}

func (e *hostEnv) getLogs() []string {
	return e.logs
}

func (e *hostEnv) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := e.ledger.Get(fullKeyHash(string(owner), string(controller), string(key)))
	return v, nil
}

func (e *hostEnv) SetValue(owner, controller, key, value []byte) error {
	e.ledger.Set(fullKeyHash(string(owner), string(controller), string(key)), value)
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
	if e.astCache == nil {
		return nil, nil
	}

	return e.astCache.GetProgram(location)
}

func (e *hostEnv) CacheProgram(location ast.Location, program *ast.Program) error {
	if e.astCache == nil {
		return nil
	}

	return e.astCache.SetProgram(location, program)
}

func (e *hostEnv) Log(message string) {
	e.logs = append(e.logs, message)
}

func (e *hostEnv) EmitEvent(event cadence.Event) {
	e.events = append(e.events, event)
}

func (e *hostEnv) GenerateUUID() uint64 {
	// TODO: https://github.com/dapperlabs/flow-go/issues/4141
	defer func() { e.uuid++ }()
	return e.uuid
}

func (e *hostEnv) GetComputationLimit() uint64 {
	if e.transactionEnv != nil {
		return e.transactionEnv.GetComputationLimit()
	}

	return e.gasLimit
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

// Block Environment Functions

// GetCurrentBlockHeight returns the current block height.
func (e *hostEnv) GetCurrentBlockHeight() uint64 {
	if e.blockHeader == nil {
		panic("GetCurrentBlockHeight is not supported by this environment")
	}

	return e.blockHeader.Height
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
	if e.blocks == nil {
		panic("GetBlockAtHeight is not supported by this environment")
	}

	block, err := e.blocks.ByHeight(height)
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
	ledger Ledger
	chain  flow.Chain
	tx     *flow.TransactionBody

	// txCtx is an execution context used to execute meta transactions
	// within this transaction context.
	txCtx Context

	authorizers                []runtime.Address
	restrictContractDeployment bool
	restrictAccountCreation    bool
}

func newTransactionEnv(
	ledger Ledger,
	chain flow.Chain,
	tx *flow.TransactionBody,
	txCtx Context,
	restrictContractDeployment bool,
	restrictAccountCreation bool,
) *transactionEnv {
	return &transactionEnv{
		ledger:                     ledger,
		chain:                      chain,
		tx:                         tx,
		txCtx:                      txCtx,
		restrictContractDeployment: restrictContractDeployment,
		restrictAccountCreation:    restrictAccountCreation,
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
	var result *InvocationResult

	result, err = e.txCtx.Invoke(
		deductAccountCreationFeeTransaction(flow.Address(payer), e.chain.ServiceAddress(), e.restrictAccountCreation),
		e.ledger,
	)
	if err != nil {
		return address, err
	}

	if result.Error != nil {
		// TODO: properly propagate this error

		switch err := result.Error.(type) {
		case *CodeExecutionError:
			return address, err.RuntimeError.Unwrap()
		default:
			// Account creation should fail due to insufficient balance, which is reported in `flowErr`.
			// Should we tree other FlowErrors as fatal?
			return address, fmt.Errorf(
				"failed to deduct account creation fee: %s",
				err.ErrorMessage(),
			)
		}
	}

	var flowAddress flow.Address

	flowAddress, err = createAccount(e.ledger, e.chain, nil)
	if err != nil {
		return address, err
	}

	result, err = e.txCtx.Invoke(initFlowTokenTransaction(flowAddress, e.chain.ServiceAddress()), e.ledger)
	if err != nil {
		return address, err
	}

	if result.Error != nil {
		// TODO: properly propagate this error

		switch err := result.Error.(type) {
		case *CodeExecutionError:
			return runtime.Address{}, err.RuntimeError.Unwrap()
		default:
			return runtime.Address{}, fmt.Errorf(
				"failed to initialize default token: %s",
				err.ErrorMessage(),
			)
		}
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
	// currently, every transaction that sets account code (deploys/updates contracts)
	// must be signed by the service account
	if e.restrictContractDeployment && !e.isValidSigningAccount(runtime.Address(e.chain.ServiceAddress())) {
		return fmt.Errorf("code deployment requires authorization from the service account")
	}

	accountAddress := flow.Address(address)

	return setAccountCode(e.ledger, accountAddress, code)
}

func (e *transactionEnv) isValidSigningAccount(address runtime.Address) bool {
	for _, accountAddress := range e.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
}
