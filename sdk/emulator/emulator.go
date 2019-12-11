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
	"github.com/dapperlabs/flow-go/sdk/abi/values"
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

	// Mutex protecting pending block
	mu sync.RWMutex

	// Pending block containing block info, register state, tx pool
	pendingBlock *types.PendingBlock

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
	SubmitTransaction(tx flow.Transaction) (types.TransactionReceipt, error)
	ExecuteScript(script []byte) (values.Value, []flow.Event, error)
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
func NewEmulatedBlockchain(opts ...Option) (*EmulatedBlockchain, error) {
	initialState := make(flow.Ledger)
	txPool := make(map[string]*flow.Transaction)
	header := &types.Block{}

	// apply options to the default config
	config := defaultConfig
	for _, opt := range opts {
		opt(&config)
	}

	// if no store is specified, use a memstore
	// NOTE: we don't initialize this in defaultConfig because otherwise the same
	// memstore instance is shared
	if config.Store == nil {
		config.Store = memstore.New()
	}
	store := config.Store

	latestBlock, err := store.GetLatestBlock()
	if err == nil && latestBlock.Number > 0 {
		// storage contains data, load state from storage
		latestState, err := store.GetLedger(latestBlock.Number)
		if err != nil {
			return nil, err
		}
		initialState = latestState

		// Restore pending block header from store information
		header = &types.Block{
			Number:            latestBlock.Number + 1,
			PreviousBlockHash: latestBlock.Hash(),
			TransactionHashes: make([]crypto.Hash, 0),
		}
	} else if err != nil && !errors.Is(err, storage.ErrNotFound{}) {
		// internal storage error, fail fast
		return nil, err
	} else {
		// storage is empty, create the root account and genesis block
		createAccount(initialState, config.RootAccountKey)

		// insert the genesis block
		genesis := types.GenesisBlock()
		if err := store.InsertBlock(genesis); err != nil {
			return nil, err
		}

		// insert the initial state containing the root account
		if err := store.SetLedger(0, initialState); err != nil {
			return nil, err
		}

		// Create pending block header from genesis block
		header = &types.Block{
			Number:            genesis.Number + 1,
			PreviousBlockHash: genesis.Hash(),
			TransactionHashes: make([]crypto.Hash, 0),
		}
	}

	pendingBlock := &types.PendingBlock{
		Header: header,
		TxPool: txPool,
		State:  initialState,
		Index:  0,
	}

	rootAccount := getAccount(initialState, flow.RootAddress)

	b := &EmulatedBlockchain{
		storage:            config.Store,
		pendingBlock:       pendingBlock,
		rootAccountAddress: rootAccount.Address,
		rootAccountKey:     config.RootAccountKey,
		lastCreatedAccount: *rootAccount,
	}

	interpreterRuntime := runtime.NewInterpreterRuntime()
	computer := execution.NewComputer(interpreterRuntime, config.RuntimeLogger)
	b.computer = computer

	return b, nil
}

// RootAccountAddress returns the root account address for this blockchain.
func (b *EmulatedBlockchain) RootAccountAddress() flow.Address {
	return b.rootAccountAddress
}

// RootKey returns the root private key for this blockchain.
func (b *EmulatedBlockchain) RootKey() flow.AccountPrivateKey {
	return b.rootAccountKey
}

// GetPendingBlock gets the pending block.
func (b *EmulatedBlockchain) GetPendingBlock() *types.PendingBlock {
	return b.pendingBlock
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
	if b.pendingBlock.ContainsTransaction(txHash) {
		return b.pendingBlock.GetTransaction(txHash), nil
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
	return getAccount(b.pendingBlock.State, address)
}

// TODO: Implement
func (b *EmulatedBlockchain) GetAccountAtBlock(address flow.Address, blockNumber uint64) (*flow.Account, error) {
	panic("not implemented")
}

func getAccount(ledger flow.Ledger, address flow.Address) *flow.Account {
	runtimeCtx := execution.NewRuntimeContext(ledger.NewView())
	return runtimeCtx.GetAccount(address)
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
func (b *EmulatedBlockchain) SubmitTransaction(tx flow.Transaction) (types.TransactionReceipt, error) {
	// Prevents tx from being executed with other transactions added previously
	if len(b.pendingBlock.Header.TransactionHashes) > 0 {
		return types.TransactionReceipt{}, &ErrPendingBlockNotEmpty{BlockHash: b.pendingBlock.Hash()}
	}

	err := b.AddTransaction(tx)
	if err != nil {
		return types.TransactionReceipt{}, err
	}

	return b.ExecuteNextTransaction()
}

// AddTransaction adds a transaction to the current pending block (validates the transaction) but holds off on executing it.
func (b *EmulatedBlockchain) AddTransaction(tx flow.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If Index > 0, pending block has begun execution (cannot add anymore txs)
	if b.pendingBlock.Index > 0 {
		return &ErrPendingBlockMidExecution{BlockHash: b.pendingBlock.Hash()}
	}

	// TODO: add more invalid transaction checks
	missingFields := tx.MissingFields()
	if len(missingFields) > 0 {
		return &ErrInvalidTransaction{TxHash: tx.Hash(), MissingFields: missingFields}
	}

	if b.pendingBlock.ContainsTransaction(tx.Hash()) {
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

	// Add tx to pending block (and transaction pool)
	tx.Status = flow.TransactionPending
	b.pendingBlock.AddTransaction(tx)

	return nil
}

// TODO: should be atomic
// ExecuteBlock executes the remaining transactions in pending block.
func (b *EmulatedBlockchain) ExecuteBlock() ([]types.TransactionReceipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	results := make([]types.TransactionReceipt, 0)

	if b.pendingBlock.Index >= len(b.pendingBlock.Header.TransactionHashes) {
		return results, &ErrPendingBlockTransactionsExhausted{BlockHash: b.pendingBlock.Hash()}
	}

	for b.pendingBlock.Index < len(b.pendingBlock.Header.TransactionHashes) {
		result, err := b.executeTransaction()
		if err != nil {
			return results, err
		}
		results = append(results, result)

	}

	return results, nil
}

// TODO: should be atomic
// ExecuteNextTransaction executes the next indexed transaction in pending block.
func (b *EmulatedBlockchain) ExecuteNextTransaction() (types.TransactionReceipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeTransaction()
}

// executeTransaction is a helper function for ExecuteBlock and ExecuteNextTransaction that runs through the transaction execution.
func (b *EmulatedBlockchain) executeTransaction() (types.TransactionReceipt, error) {
	// Check if there are remaining txs to be executed
	if b.pendingBlock.Index >= len(b.pendingBlock.Header.TransactionHashes) {
		return types.TransactionReceipt{}, &ErrPendingBlockTransactionsExhausted{BlockHash: b.pendingBlock.Hash()}
	}

	txHash := b.pendingBlock.Header.TransactionHashes[b.pendingBlock.Index]
	if !b.pendingBlock.ContainsTransaction(txHash) {
		return types.TransactionReceipt{}, &ErrTransactionNotFound{TxHash: txHash}
	}
	tx := b.pendingBlock.GetTransaction(txHash)

	// Advances the transaction list index (inside a block)
	// Note: we want to advance the index even if tx reverts
	b.pendingBlock.Index++

	ledger := b.pendingBlock.State.NewView()

	events, err := b.computer.ExecuteTransaction(ledger, *tx)
	if err != nil {
		tx.Status = flow.TransactionReverted

		receipt := types.TransactionReceipt{
			TransactionHash: txHash,
			Status:          tx.Status,
			Error:           err,
		}

		return receipt, nil
	}

	// Update pending state with ledger changed during transaction execution
	b.pendingBlock.State.MergeWith(ledger.Updated())

	// Update the transaction's status and events
	// NOTE: this updates txPool state because txPool stores pointers
	tx.Status = flow.TransactionFinalized
	tx.Events = events

	// TODO: improve the pending block, provide all block information
	prevBlock, err := b.storage.GetLatestBlock()
	if err != nil {
		return types.TransactionReceipt{}, fmt.Errorf("failed to get latest block: %w", err)
	}
	blockNumber := prevBlock.Number + 1

	// Update system state based on emitted events
	b.handleEvents(events, blockNumber, tx.Hash())

	// TODO: Do this in CommitBlock instead
	if err := b.storage.InsertEvents(blockNumber, events...); err != nil {
		return types.TransactionReceipt{}, fmt.Errorf("failed to insert events: %w", err)
	}

	receipt := types.TransactionReceipt{
		TransactionHash: txHash,
		Status:          tx.Status,
		Error:           nil,
		Events:          tx.Events,
	}

	return receipt, nil
}

// CommitBlock seals the current pending block and saves it to storage.
//
// Note: this clears the pending transaction pool and resets the pending block.
// This also indexes the committed blockchain state for testing purposes.
func (b *EmulatedBlockchain) CommitBlock() (*types.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Pending block cannot be committed before execution
	if b.pendingBlock.Index == 0 && len(b.pendingBlock.Header.TransactionHashes) > 0 {
		return nil, &ErrPendingBlockCommitBeforeExecution{BlockHash: b.pendingBlock.Hash()}
	}

	// Pending block has begun execution but has not finished (cannot commit state)
	if b.pendingBlock.Index > 0 && b.pendingBlock.Index < len(b.pendingBlock.Header.TransactionHashes) {
		return nil, &ErrPendingBlockMidExecution{BlockHash: b.pendingBlock.Hash()}
	}

	for _, txHash := range b.pendingBlock.Header.TransactionHashes {
		if b.pendingBlock.ContainsTransaction(txHash) {
			tx := b.pendingBlock.GetTransaction(txHash)
			// Update transactions to TransactionSealed status
			if tx.Status != flow.TransactionReverted {
				tx.Status = flow.TransactionSealed
			}
			if err := b.storage.InsertTransaction(*tx); err != nil {
				return nil, &ErrStorage{err}
			}
		} else {
			return nil, &ErrTransactionNotFound{TxHash: txHash}
		}

	}

	b.pendingBlock.Header.Timestamp = time.Now()

	if err := b.storage.InsertBlock(*b.pendingBlock.Header); err != nil {
		return nil, &ErrStorage{err}
	}

	if err := b.storage.SetLedger(b.pendingBlock.Header.Number, b.pendingBlock.State); err != nil {
		return nil, &ErrStorage{err}
	}

	// Grab reference to pending block to return at the end
	block := b.pendingBlock.Header

	// Reset pending block header
	b.pendingBlock.ResetHeader(block)

	// Clear transaction pool
	b.pendingBlock.ClearTxPool()

	return block, nil
}

// TODO: should be atomic
// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitState.
func (b *EmulatedBlockchain) ExecuteAndCommitBlock() (*types.Block, error) {
	_, err := b.ExecuteBlock()
	if err != nil {
		return nil, err
	}

	block, err := b.CommitBlock()
	if err != nil {
		return nil, err
	}

	return block, nil
}

// ResetPendingBlock clears the transactions in pending block.
func (b *EmulatedBlockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	prevBlock, err := b.GetLatestBlock()
	if err != nil {
		return err
	}

	// Reset pending state
	prevState, err := b.storage.GetLedger(prevBlock.Number)
	if err != nil {
		return err
	}
	b.pendingBlock.ResetState(prevState)

	// Reset pending block header
	b.pendingBlock.ResetHeader(prevBlock)

	// Clear transaction pool
	b.pendingBlock.ClearTxPool()

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) ExecuteScript(script []byte) (values.Value, []flow.Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ledger := b.pendingBlock.State.NewView()
	return b.computer.ExecuteScript(ledger, script)
}

// TODO: implement
func (b *EmulatedBlockchain) ExecuteScriptAtBlock(script []byte, blockNumber uint64) (interface{}, []flow.Event, error) {
	panic("not implemented")
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

	_, err = b.SubmitTransaction(tx)
	if err != nil {
		return flow.Address{}, err
	}

	_, err = b.CommitBlock()
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
			acctCreatedEvent, err := flow.DecodeAccountCreatedEvent(event.Payload)
			if err != nil {
				panic("failed to decode AccountCreated event")
			}

			address := acctCreatedEvent.Address()

			account := b.getAccount(address)
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
		[]values.Bytes{publicKeyBytes},
	)
	if err != nil {
		panic(err)
	}

	ledger.MergeWith(view.Updated())

	account := runtimeContext.GetAccount(flow.Address(accountAddress))
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
}
