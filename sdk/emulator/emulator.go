package emulator

import (
	"errors"
	"fmt"
	"sync"

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
	lastCreatedAddress flow.Address
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
	var pendingBlock *types.PendingBlock
	var rootAccount *flow.Account

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

		// Restore pending block header from store information
		pendingBlock = types.NewPendingBlock(latestBlock, latestState)
		rootAccount = getAccount(latestState.NewView(), flow.RootAddress)

	} else if err != nil && !errors.Is(err, storage.ErrNotFound{}) {
		// internal storage error, fail fast
		return nil, err
	} else {
		genesisState := make(flow.Ledger)

		// storage is empty, create the root account and genesis block
		createAccount(genesisState, config.RootAccountKey)

		// insert the genesis block
		genesis := types.GenesisBlock()
		if err := store.InsertBlock(genesis); err != nil {
			return nil, err
		}

		// insert the initial state containing the root account
		if err := store.SetLedger(0, genesisState); err != nil {
			return nil, err
		}

		// Create pending block header from genesis block
		pendingBlock = types.NewPendingBlock(genesis, genesisState)
		rootAccount = getAccount(genesisState.NewView(), flow.RootAddress)
	}

	b := &EmulatedBlockchain{
		storage:            config.Store,
		pendingBlock:       pendingBlock,
		rootAccountAddress: rootAccount.Address,
		rootAccountKey:     config.RootAccountKey,
		lastCreatedAddress: rootAccount.Address,
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
// First looks in pending block, then looks in current blockchain state.
func (b *EmulatedBlockchain) GetTransaction(txHash crypto.Hash) (*flow.Transaction, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	pendingTx := b.pendingBlock.GetTransaction(txHash)
	if pendingTx != nil {
		return pendingTx, nil
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

// GetAccount returns the account for the given address.
func (b *EmulatedBlockchain) GetAccount(address flow.Address) (*flow.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.getAccount(address)
}

// getAccount returns the account for the given address.
func (b *EmulatedBlockchain) getAccount(address flow.Address) (*flow.Account, error) {
	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	latestLedger, err := b.storage.GetLedger(latestBlock.Number)
	if err != nil {
		return nil, err
	}

	acct := getAccount(latestLedger.NewView(), address)
	if acct == nil {
		return nil, &ErrAccountNotFound{Address: address}
	}

	return acct, nil
}

// TODO: Implement
func (b *EmulatedBlockchain) GetAccountAtBlock(address flow.Address, blockNumber uint64) (*flow.Account, error) {
	panic("not implemented")
}

func getAccount(ledger *flow.LedgerView, address flow.Address) *flow.Account {
	runtimeCtx := execution.NewRuntimeContext(ledger)
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
	b.mu.Lock()
	defer b.mu.Unlock()

	// Prevents tx from being executed with other transactions added previously
	if b.pendingBlock.TransactionCount() > 0 {
		return types.TransactionReceipt{}, &ErrPendingBlockNotEmpty{BlockHash: b.pendingBlock.Hash()}
	}

	err := b.addTransaction(tx)
	if err != nil {
		return types.TransactionReceipt{}, err
	}

	return b.executeTransaction()
}

// AddTransaction adds a transaction to the current pending block (validates the transaction) but holds off on executing it.
func (b *EmulatedBlockchain) AddTransaction(tx flow.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.addTransaction(tx)
}

func (b *EmulatedBlockchain) addTransaction(tx flow.Transaction) error {
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

	return b.executeBlock()
}

func (b *EmulatedBlockchain) executeBlock() ([]types.TransactionReceipt, error) {
	results := make([]types.TransactionReceipt, 0)

	if b.pendingBlock.Index >= b.pendingBlock.TransactionCount() {
		return results, &ErrPendingBlockTransactionsExhausted{BlockHash: b.pendingBlock.Hash()}
	}

	for b.pendingBlock.Index < b.pendingBlock.TransactionCount() {
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
	if b.pendingBlock.Index >= b.pendingBlock.TransactionCount() {
		return types.TransactionReceipt{}, &ErrPendingBlockTransactionsExhausted{BlockHash: b.pendingBlock.Hash()}
	}

	var receipt types.TransactionReceipt

	b.pendingBlock.ExecuteNextTransaction(
		func(
			tx *flow.Transaction,
			ledger *flow.LedgerView,
			success func(events []flow.Event),
			revert func(),
		) {
			events, err := b.computer.ExecuteTransaction(ledger, *tx)
			if err != nil {
				receipt = types.TransactionReceipt{
					TransactionHash: tx.Hash(),
					Status:          flow.TransactionReverted,
					Error:           err,
				}

				revert()
				return
			}

			receipt = types.TransactionReceipt{
				TransactionHash: tx.Hash(),
				Status:          flow.TransactionFinalized,
				Error:           nil,
				Events:          events,
			}

			success(events)
		},
	)

	return receipt, nil
}

// CommitBlock seals the current pending block and saves it to storage.
//
// Note: this clears the pending transaction pool and resets the pending block.
// This also indexes the committed blockchain state for testing purposes.
func (b *EmulatedBlockchain) CommitBlock() (*types.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.commitBlock()
}

func (b *EmulatedBlockchain) commitBlock() (*types.Block, error) {
	// Pending block cannot be committed before execution
	if b.pendingBlock.Index == 0 && b.pendingBlock.TransactionCount() > 0 {
		return nil, &ErrPendingBlockCommitBeforeExecution{BlockHash: b.pendingBlock.Hash()}
	}

	// Pending block has begun execution but has not finished (cannot commit state)
	if b.pendingBlock.Index > 0 && b.pendingBlock.Index < b.pendingBlock.TransactionCount() {
		return nil, &ErrPendingBlockMidExecution{BlockHash: b.pendingBlock.Hash()}
	}

	err := b.storage.CommitPendingBlock(b.pendingBlock)
	if err != nil {
		return nil, err
	}

	// Update system state based on emitted events
	b.handleEvents(b.pendingBlock.Events(), b.pendingBlock.Header.Number)

	// Grab reference to pending block to return at the end
	block := b.pendingBlock.Header

	// Reset pending block using current block and current register state
	b.pendingBlock = types.NewPendingBlock(*block, b.pendingBlock.State)

	return block, nil
}

// TODO: should be atomic
// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *EmulatedBlockchain) ExecuteAndCommitBlock() (*types.Block, []types.TransactionReceipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	results, err := b.executeBlock()
	if err != nil {
		return nil, nil, err
	}

	block, err := b.commitBlock()
	if err != nil {
		return nil, results, err
	}

	return block, results, nil
}

// ResetPendingBlock clears the transactions in pending block.
func (b *EmulatedBlockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return err
	}

	latestLedger, err := b.storage.GetLedger(latestBlock.Number)
	if err != nil {
		return err
	}

	// Reset pending block using latest committed block and state
	b.pendingBlock = types.NewPendingBlock(*latestBlock, latestLedger)

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) ExecuteScript(script []byte) (values.Value, []flow.Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, nil, err
	}

	latestLedger, err := b.storage.GetLedger(latestBlock.Number)
	if err != nil {
		return nil, nil, err
	}

	return b.computer.ExecuteScript(latestLedger.NewView(), script)
}

// TODO: implement
func (b *EmulatedBlockchain) ExecuteScriptAtBlock(script []byte, blockNumber uint64) (interface{}, []flow.Event, error) {
	panic("not implemented")
}

// LastCreatedAccount returns the last account that was created in the blockchain.
func (b *EmulatedBlockchain) LastCreatedAccount() flow.Account {
	account, _ := b.getAccount(b.lastCreatedAddress)
	return *account
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
		return flow.Address{}, err
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
		return flow.Address{}, err
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

// UpdateAccountCode submits a transaction to update the code of an existing account
// with the given code. The transaction is paid by the root account.
func (b *EmulatedBlockchain) UpdateAccountCode(
	code []byte, nonce uint64,
) error {
	createAccountScript := templates.UpdateAccountCode(code)

	tx := flow.Transaction{
		Script:             createAccountScript,
		ReferenceBlockHash: nil,
		Nonce:              nonce,
		ComputeLimit:       10,
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		PayerAccount:       b.RootAccountAddress(),
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	if err != nil {
		return nil
	}

	tx.AddSignature(b.RootAccountAddress(), sig)

	_, err = b.SubmitTransaction(tx)
	if err != nil {
		return err
	}

	_, err = b.CommitBlock()
	if err != nil {
		return err
	}

	return nil
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
	account, err := b.getAccount(accountSig.Account)
	if err != nil {
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
func (b *EmulatedBlockchain) handleEvents(events []flow.Event, blockNumber uint64) {
	for _, event := range events {
		// update lastCreatedAccount if this is an AccountCreated event
		if event.Type == flow.EventAccountCreated {
			acctCreatedEvent, err := flow.DecodeAccountCreatedEvent(event.Payload)
			if err != nil {
				panic("failed to decode AccountCreated event")
			}

			b.lastCreatedAddress = acctCreatedEvent.Address()
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
