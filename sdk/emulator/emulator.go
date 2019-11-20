package emulator

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage/memstore"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

// EmulatedBlockchain simulates a blockchain for testing purposes.
//
// An emulated blockchain contains a versioned world state store and a pending
// transaction pool for granular state update tests.
//
// The intermediate world state is stored after each transaction is executed
// and can be used to check the output of a single transaction.
//
// The final world state is committed after each block. An index of committed
// world states is maintained to allow a test to seek to a previously committed
// world state.
type EmulatedBlockchain struct {
	// Finalized chain state: blocks, transactions, registers, events
	storage storage.Store

	// Mutex protecting pending register state and txPool
	mu sync.RWMutex
	// The current working register state, up-to-date with all transactions
	// in the txPool.
	pendingState flow.Ledger
	// Pool of transactions that have been executed, but not finalized
	txPool map[string]*flow.Transaction

	// The runtime context used to execute transactions and scripts
	computer *execution.Computer

	rootAccountAddress flow.Address
	rootAccountKey     flow.AccountPrivateKey
	lastCreatedAccount flow.Account
}

// EmulatedBlockchainAPI defines the method set of EmulatedBlockchain.
type EmulatedBlockchainAPI interface {
	RootAccountAddress() flow.Address
	RootKey() flow.AccountPrivateKey
	GetLatestBlock() (*types.Block, error)
	GetBlockByHash(hash crypto.Hash) (*types.Block, error)
	GetBlockByNumber(number uint64) (*types.Block, error)
	GetTransaction(txHash crypto.Hash) (*flow.Transaction, error)
	GetAccount(address flow.Address) (*flow.Account, error)
	GetAccountAtBlock(address flow.Address, blockNumber uint64) (*flow.Account, error)
	GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error)
	SubmitTransaction(tx flow.Transaction) error
	ExecuteScript(script []byte) (interface{}, []flow.Event, error)
	ExecuteScriptAtBlock(script []byte, blockNumber uint64) (interface{}, []flow.Event, error)
	CommitBlock() (*types.Block, error)
	LastCreatedAccount() flow.Account
	CreateAccount(
		publicKeys []flow.AccountPublicKey,
		code []byte, nonce uint64,
	) (flow.Address, error)
}

// Config is a set of configuration options for an emulated blockchain.
type Config struct {
	RootAccountKey flow.AccountPrivateKey
	RuntimeLogger  func(string)
	Store          storage.Store
}

// defaultConfig is the default configuration for an emulated blockchain.
// NOTE: Instantiated in init function
var defaultConfig Config

// Option is a function applying a change to the emulator config.
type Option func(*Config)

// WithRootKey sets the root key.
func WithRootAccountKey(rootKey flow.AccountPrivateKey) Option {
	return func(c *Config) {
		c.RootAccountKey = rootKey
	}
}

// WithRuntimeLogger sets the runtime logger handler function.
func WithRuntimeLogger(logger func(string)) Option {
	return func(c *Config) {
		c.RuntimeLogger = logger
	}
}

// WithStore sets the persistent storage provider.
func WithStore(store storage.Store) Option {
	return func(c *Config) {
		c.Store = store
	}
}

// NewEmulatedBlockchain instantiates a new blockchain backend for testing purposes.
func NewEmulatedBlockchain(opts ...Option) *EmulatedBlockchain {
	initialState := make(flow.Ledger)
	txPool := make(map[string]*flow.Transaction)

	// apply options to the default config
	config := defaultConfig
	for _, opt := range opts {
		opt(&config)
	}

	// create the root account
	rootAccount := createAccount(initialState, config.RootAccountKey)

	b := &EmulatedBlockchain{
		storage:            config.Store,
		pendingState:       initialState,
		txPool:             txPool,
		rootAccountAddress: rootAccount.Address,
		rootAccountKey:     config.RootAccountKey,
		lastCreatedAccount: rootAccount,
	}

	interpreterRuntime := runtime.NewInterpreterRuntime()
	computer := execution.NewComputer(interpreterRuntime, config.RuntimeLogger)
	b.computer = computer

	return b
}

// RootAccountAddress returns the root account address for this blockchain.
func (b *EmulatedBlockchain) RootAccountAddress() flow.Address {
	return b.rootAccountAddress
}

// RootKey returns the root private key for this blockchain.
func (b *EmulatedBlockchain) RootKey() flow.AccountPrivateKey {
	return b.rootAccountKey
}

// GetLatestBlock gets the latest sealed block.
func (b *EmulatedBlockchain) GetLatestBlock() (*types.Block, error) {
	block, err := b.storage.GetLatestBlock()
	if err != nil {
		return nil, &ErrStorage{err}
	}
	return &block, nil
}

// GetBlockByHash gets a block by hash.
func (b *EmulatedBlockchain) GetBlockByHash(hash crypto.Hash) (*types.Block, error) {
	block, err := b.storage.GetBlockByHash(hash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound{}) {
			return nil, &ErrBlockNotFound{BlockHash: hash}
		}
		return nil, &ErrStorage{err}
	}

	return &block, nil
}

// GetBlockByNumber gets a block by number.
func (b *EmulatedBlockchain) GetBlockByNumber(number uint64) (*types.Block, error) {
	block, err := b.storage.GetBlockByNumber(number)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound{}) {
			return nil, &ErrBlockNotFound{BlockNum: number}
		}
		return nil, err
	}

	return &block, nil
}

// GetTransaction gets an existing transaction by hash.
//
// First looks in pending txPool, then looks in current blockchain state.
func (b *EmulatedBlockchain) GetTransaction(txHash crypto.Hash) (*flow.Transaction, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if tx, ok := b.txPool[string(txHash)]; ok {
		return tx, nil
	}

	tx, err := b.storage.GetTransaction(txHash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound{}) {
			return nil, &ErrTransactionNotFound{TxHash: txHash}
		}
		return nil, &ErrStorage{err}
	}

	return &tx, nil
}

// GetAccount gets account information associated with an address identifier.
func (b *EmulatedBlockchain) GetAccount(address flow.Address) (*flow.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	acct := b.getAccount(address)
	if acct == nil {
		return nil, &ErrAccountNotFound{Address: address}
	}
	return acct, nil
}

// Returns the account for the given address, or nil if the account does not
// exist.
func (b *EmulatedBlockchain) getAccount(address flow.Address) *flow.Account {
	ledger := b.pendingState
	runtimeContext := execution.NewRuntimeContext(ledger.NewView())
	return runtimeContext.GetAccount(address)
}

// TODO: Implement
func (b *EmulatedBlockchain) GetAccountAtBlock(address flow.Address, blockNumber uint64) (*flow.Account, error) {
	panic("not implemented")
}

// GetEvents returns events matching a query.
func (b *EmulatedBlockchain) GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
	return b.storage.GetEvents(eventType, startBlock, endBlock)
}

// SubmitTransaction sends a transaction to the network that is immediately
// executed (updates pending blockchain state).
//
// Note that the resulting state is not finalized until CommitBlock() is called.
// However, the pending blockchain state is indexed for testing purposes.
func (b *EmulatedBlockchain) SubmitTransaction(tx flow.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// TODO: add more invalid transaction checks
	missingFields := tx.MissingFields()
	if len(missingFields) > 0 {
		return &ErrInvalidTransaction{TxHash: tx.Hash(), MissingFields: missingFields}
	}

	if _, exists := b.txPool[string(tx.Hash())]; exists {
		return &ErrDuplicateTransaction{TxHash: tx.Hash()}
	}

	_, err := b.storage.GetTransaction(tx.Hash())
	if err == nil {
		// Found the transaction, this is a dupe
		return &ErrDuplicateTransaction{TxHash: tx.Hash()}
	} else if !errors.Is(err, storage.ErrNotFound{}) {
		// Error in the storage provider
		return fmt.Errorf("failed to check storage for transaction %w", err)
	}

	if err := b.verifySignatures(tx); err != nil {
		return err
	}

	tx.Status = flow.TransactionPending
	b.txPool[string(tx.Hash())] = &tx

	ledger := b.pendingState.NewView()

	events, err := b.computer.ExecuteTransaction(ledger, tx)
	if err != nil {
		tx.Status = flow.TransactionReverted
		return &ErrTransactionReverted{TxHash: tx.Hash(), Err: err}
	}

	// Update pending state with ledger changed during transaction execution
	b.pendingState.MergeWith(ledger.Updated())

	// Update the transaction's status and events
	// NOTE: this updates txPool state because txPool stores pointers
	tx.Status = flow.TransactionFinalized
	tx.Events = events

	// TODO: improve the pending block, provide all block information
	prevBlock, err := b.storage.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}
	blockNumber := prevBlock.Number + 1

	// Update system state based on emitted events
	b.handleEvents(events, blockNumber, tx.Hash())

	// TODO: Do this in CommitBlock instead
	if err := b.storage.InsertEvents(blockNumber, events...); err != nil {
		return fmt.Errorf("failed to insert events: %w", err)
	}

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) ExecuteScript(script []byte) (interface{}, []flow.Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ledger := b.pendingState.NewView()
	value, events, err := b.computer.ExecuteScript(ledger, script)
	if err != nil {
		return nil, nil, err
	}

	return value, events, nil
}

// TODO: implement
func (b *EmulatedBlockchain) ExecuteScriptAtBlock(script []byte, blockNumber uint64) (interface{}, []flow.Event, error) {
	panic("not implemented")
}

// CommitBlock takes all pending transactions and commits them into a block.
//
// Note: this clears the pending transaction pool and indexes the committed blockchain
// state for testing purposes.
func (b *EmulatedBlockchain) CommitBlock() (*types.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	txHashes := make([]crypto.Hash, 0)
	for _, tx := range b.txPool {
		txHashes = append(txHashes, tx.Hash())
		if tx.Status != flow.TransactionReverted {
			tx.Status = flow.TransactionSealed
		}
	}

	prevBlock, err := b.storage.GetLatestBlock()
	if err != nil {
		return nil, &ErrStorage{err}
	}
	block := &types.Block{
		Number:            prevBlock.Number + 1,
		Timestamp:         time.Now(),
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: txHashes,
	}

	for _, tx := range b.txPool {
		if err := b.storage.InsertTransaction(*tx); err != nil {
			return nil, &ErrStorage{err}
		}
	}

	if err := b.storage.InsertBlock(*block); err != nil {
		return nil, &ErrStorage{err}
	}

	if err := b.storage.SetLedger(block.Number, b.pendingState); err != nil {
		return nil, &ErrStorage{err}
	}

	// reset tx pool
	b.txPool = make(map[string]*flow.Transaction)

	return block, nil
}

// LastCreatedAccount returns the last account that was created in the blockchain.
func (b *EmulatedBlockchain) LastCreatedAccount() flow.Account {
	return b.lastCreatedAccount
}

// verifySignatures verifies that a transaction contains the necessary signatures.
//
// An error is returned if any of the expected signatures are invalid or missing.
func (b *EmulatedBlockchain) verifySignatures(tx flow.Transaction) error {
	accountWeights := make(map[flow.Address]int)

	encodedTx := tx.Encode()

	for _, accountSig := range tx.Signatures {
		accountPublicKey, err := b.verifyAccountSignature(accountSig, encodedTx)
		if err != nil {
			return err
		}

		accountWeights[accountSig.Account] += accountPublicKey.Weight
	}

	if accountWeights[tx.PayerAccount] < keys.PublicKeyWeightThreshold {
		return &ErrMissingSignature{tx.PayerAccount}
	}

	for _, account := range tx.ScriptAccounts {
		if accountWeights[account] < keys.PublicKeyWeightThreshold {
			return &ErrMissingSignature{account}
		}
	}

	return nil
}

// CreateAccount submits a transaction to create a new account with the given
// account keys and code. The transaction is paid by the root account.
func (b *EmulatedBlockchain) CreateAccount(
	publicKeys []flow.AccountPublicKey,
	code []byte, nonce uint64,
) (flow.Address, error) {
	createAccountScript, err := templates.CreateAccount(publicKeys, code)

	if err != nil {
		return flow.Address{}, nil
	}

	tx := flow.Transaction{
		Script:             createAccountScript,
		ReferenceBlockHash: nil,
		Nonce:              nonce,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	if err != nil {
		return flow.Address{}, nil
	}

	tx.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx)
	if err != nil {
		return flow.Address{}, err
	}

	return b.LastCreatedAccount().Address, nil
}

// verifyAccountSignature verifies that an account signature is valid for the
// account and given message.
//
// If the signature is valid, this function returns the associated account key.
//
// An error is returned if the account does not contain a public key that
// correctly verifies the signature against the given message.
func (b *EmulatedBlockchain) verifyAccountSignature(
	accountSig flow.AccountSignature,
	message []byte,
) (accountPublicKey flow.AccountPublicKey, err error) {
	account := b.getAccount(accountSig.Account)
	if account == nil {
		return accountPublicKey, &ErrInvalidSignatureAccount{Account: accountSig.Account}
	}

	signature := crypto.Signature(accountSig.Signature)

	// TODO: account signatures should specify a public key (possibly by index) to avoid this loop
	for _, accountPublicKey := range account.Keys {
		hasher, _ := crypto.NewHasher(accountPublicKey.HashAlgo)

		valid, err := accountPublicKey.PublicKey.Verify(signature, message, hasher)
		if err != nil {
			continue
		}

		if valid {
			return accountPublicKey, nil
		}
	}

	return accountPublicKey, &ErrInvalidSignaturePublicKey{
		Account: accountSig.Account,
	}
}

// handleEvents updates emulator state based on emitted system events.
func (b *EmulatedBlockchain) handleEvents(events []flow.Event, blockNumber uint64, txHash crypto.Hash) {
	for _, event := range events {
		// update lastCreatedAccount if this is an AccountCreated event
		if event.Type == flow.EventAccountCreated {
			accountAddress := event.Values["address"].(flow.Address)

			account := b.getAccount(accountAddress)
			if account == nil {
				panic("failed to get newly-created account")
			}

			b.lastCreatedAccount = *account
		}

	}
}

// createAccount creates an account with the given private key and injects it
// into the given state, bypassing the need for a transaction.
func createAccount(ledger flow.Ledger, privateKey flow.AccountPrivateKey) flow.Account {
	publicKey := privateKey.PublicKey(keys.PublicKeyWeightThreshold)
	publicKeyBytes, err := flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	view := ledger.NewView()
	runtimeContext := execution.NewRuntimeContext(view)
	accountAddress, err := runtimeContext.CreateAccount(
		[][]byte{publicKeyBytes},
		[]byte{},
	)
	if err != nil {
		panic(err)
	}

	ledger.MergeWith(view.Updated())

	account := runtimeContext.GetAccount(accountAddress)
	return *account
}

func init() {
	// Initialize default emulator options
	defaultRootKey, err := keys.GeneratePrivateKey(
		keys.ECDSA_P256_SHA3_256,
		[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
	if err != nil {
		panic("Failed to generate default root key: " + err.Error())
	}

	defaultConfig.RuntimeLogger = func(string) {}
	defaultConfig.RootAccountKey = defaultRootKey
	defaultConfig.Store = memstore.New()
}
