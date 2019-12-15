// Package emulator provides an emulated version of the Flow blockchain that can be used
// for development purposes.
//
// This package can be used as a library or as a standalone application.
//
// When used as a library, this package provides tools to write programmatic tests for
// Flow applications.
//
// When used as a standalone application, this package implements the Flow Observation API
// and is fully-compatible with Flow gRPC client libraries.
package emulator

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/emulator/storage"
	"github.com/dapperlabs/flow-go/sdk/emulator/storage/memstore"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

// Blockchain emulates the functionality of the Flow blockchain.
type Blockchain struct {
	// committed chain state: blocks, transactions, registers, events
	storage storage.Store

	// mutex protecting pending block
	mu sync.RWMutex

	// pending block containing block info, register state, pending transactions
	pendingBlock *pendingBlock

	// runtime context used to execute transactions and scripts
	computer *computer

	rootAccountAddress flow.Address
	rootAccountKey     flow.AccountPrivateKey
	lastCreatedAddress flow.Address
}

// BlockchainAPI defines the method set of an emulated blockchain.
type BlockchainAPI interface {
	RootAccountAddress() flow.Address
	RootKey() flow.AccountPrivateKey
	GetLatestBlock() (*types.Block, error)
	GetBlockByHash(hash crypto.Hash) (*types.Block, error)
	GetBlockByNumber(number uint64) (*types.Block, error)
	GetTransaction(txHash crypto.Hash) (*flow.Transaction, error)
	GetAccount(address flow.Address) (*flow.Account, error)
	GetAccountAtBlock(address flow.Address, blockNumber uint64) (*flow.Account, error)
	GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error)
	AddTransaction(tx flow.Transaction) error
	ExecuteScript(script []byte) (ScriptResult, error)
	ExecuteScriptAtBlock(script []byte, blockNumber uint64) (ScriptResult, error)
	CommitBlock() (*types.Block, error)
	ExecuteAndCommitBlock() (*types.Block, []TransactionResult, error)
}

// Config is a set of configuration options for an emulated blockchain.
type Config struct {
	RootAccountKey flow.AccountPrivateKey
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

// WithStore sets the persistent storage provider.
func WithStore(store storage.Store) Option {
	return func(c *Config) {
		c.Store = store
	}
}

// NewBlockchain instantiates a new emulated blockchain with the provided options.
func NewBlockchain(opts ...Option) (*Blockchain, error) {
	var pendingBlock *pendingBlock
	var rootAccount *flow.Account

	// apply options to the default config
	config := defaultConfig
	for _, opt := range opts {
		opt(&config)
	}

	// if no store is specified, use a memstore
	// NOTE: we don't initialize this in defaultConfig because otherwise the same
	// memstore is shared between Blockchain instances
	if config.Store == nil {
		config.Store = memstore.New()
	}
	store := config.Store

	latestBlock, err := store.LatestBlock()
	if err == nil && latestBlock.Number > 0 {
		// storage contains data, load state from storage
		latestLedger, err := store.LedgerByNumber(latestBlock.Number)
		if err != nil {
			return nil, err
		}

		// restore pending block header from store information
		pendingBlock = newPendingBlock(latestBlock, latestLedger)
		rootAccount = getAccount(latestLedger.NewView(), flow.RootAddress)

	} else if err != nil && !errors.Is(err, storage.ErrNotFound{}) {
		// internal storage error, fail fast
		return nil, err
	} else {
		genesisLedger := make(flow.Ledger)

		// storage is empty, create the root account and genesis block
		createAccount(genesisLedger, config.RootAccountKey)

		// insert the genesis block
		genesis := types.GenesisBlock()
		if err := store.InsertBlock(genesis); err != nil {
			return nil, err
		}

		// insert the initial state containing the root account
		if err := store.InsertLedger(0, genesisLedger); err != nil {
			return nil, err
		}

		// create pending block header from genesis block
		pendingBlock = newPendingBlock(genesis, genesisLedger)
		rootAccount = getAccount(genesisLedger.NewView(), flow.RootAddress)
	}

	b := &Blockchain{
		storage:            config.Store,
		pendingBlock:       pendingBlock,
		rootAccountAddress: rootAccount.Address,
		rootAccountKey:     config.RootAccountKey,
		lastCreatedAddress: rootAccount.Address,
	}

	interpreterRuntime := runtime.NewInterpreterRuntime()
	b.computer = newComputer(interpreterRuntime)

	return b, nil
}

// RootAccountAddress returns the root account address for this blockchain.
func (b *Blockchain) RootAccountAddress() flow.Address {
	return b.rootAccountAddress
}

// RootKey returns the root private key for this blockchain.
func (b *Blockchain) RootKey() flow.AccountPrivateKey {
	return b.rootAccountKey
}

// PendingBlock returns the hash of the pending block.
func (b *Blockchain) PendingBlockHash() crypto.Hash {
	return b.pendingBlock.Hash()
}

// GetLatestBlock gets the latest sealed block.
func (b *Blockchain) GetLatestBlock() (*types.Block, error) {
	block, err := b.storage.LatestBlock()
	if err != nil {
		return nil, &ErrStorage{err}
	}
	return &block, nil
}

// GetBlockByHash gets a block by hash.
func (b *Blockchain) GetBlockByHash(hash crypto.Hash) (*types.Block, error) {
	block, err := b.storage.BlockByHash(hash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound{}) {
			return nil, &ErrBlockNotFound{BlockHash: hash}
		}
		return nil, &ErrStorage{err}
	}

	return &block, nil
}

// GetBlockByNumber gets a block by number.
func (b *Blockchain) GetBlockByNumber(number uint64) (*types.Block, error) {
	block, err := b.storage.BlockByNumber(number)
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
// The function first looks in the pending block, then the current blockchain state.
func (b *Blockchain) GetTransaction(txHash crypto.Hash) (*flow.Transaction, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	pendingTx := b.pendingBlock.GetTransaction(txHash)
	if pendingTx != nil {
		return pendingTx, nil
	}

	tx, err := b.storage.TransactionByHash(txHash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound{}) {
			return nil, &ErrTransactionNotFound{TxHash: txHash}
		}
		return nil, &ErrStorage{err}
	}

	return &tx, nil
}

// GetAccount returns the account for the given address.
func (b *Blockchain) GetAccount(address flow.Address) (*flow.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.getAccount(address)
}

// getAccount returns the account for the given address.
func (b *Blockchain) getAccount(address flow.Address) (*flow.Account, error) {
	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	latestLedger, err := b.storage.LedgerByNumber(latestBlock.Number)
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
func (b *Blockchain) GetAccountAtBlock(address flow.Address, blockNumber uint64) (*flow.Account, error) {
	panic("not implemented")
}

func getAccount(ledger *flow.LedgerView, address flow.Address) *flow.Account {
	runtimeCtx := execution.NewRuntimeContext(ledger)
	return runtimeCtx.GetAccount(address)
}

// GetEvents returns events matching a query.
func (b *Blockchain) GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
	return b.storage.RetrieveEvents(eventType, startBlock, endBlock)
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) AddTransaction(tx flow.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If Index > 0, pending block has begun execution (cannot add anymore txs)
	if b.pendingBlock.ExecutionStarted() {
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

	_, err := b.storage.TransactionByHash(tx.Hash())
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

	// add transaction to pending block
	tx.Status = flow.TransactionPending
	b.pendingBlock.AddTransaction(tx)

	return nil
}

// ExecuteBlock executes the remaining transactions in pending block.
func (b *Blockchain) ExecuteBlock() ([]TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeBlock()
}

func (b *Blockchain) executeBlock() ([]TransactionResult, error) {
	results := make([]TransactionResult, 0)

	// empty blocks do not require execution, treat as a no-op
	if b.pendingBlock.Empty() {
		return results, nil
	}

	// cannot execute a block that has already executed
	if b.pendingBlock.ExecutionComplete() {
		return results, &ErrPendingBlockTransactionsExhausted{
			BlockHash: b.pendingBlock.Hash(),
		}
	}

	// continue executing transactions until execution is complete
	for !b.pendingBlock.ExecutionComplete() {
		result, err := b.executeNextTransaction()
		if err != nil {
			return results, err
		}

		results = append(results, result)
	}

	return results, nil
}

// ExecuteNextTransaction executes the next indexed transaction in pending block.
func (b *Blockchain) ExecuteNextTransaction() (TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeNextTransaction()
}

// executeNextTransaction is a helper function for ExecuteBlock and ExecuteNextTransaction that
// executes the next transaction in the pending block.
func (b *Blockchain) executeNextTransaction() (TransactionResult, error) {
	// check if there are remaining txs to be executed
	if b.pendingBlock.ExecutionComplete() {
		return TransactionResult{}, &ErrPendingBlockTransactionsExhausted{
			BlockHash: b.pendingBlock.Hash(),
		}
	}

	// use the computer to execute the next transaction
	receipt, err := b.pendingBlock.ExecuteNextTransaction(
		func(
			ledger *flow.LedgerView,
			tx flow.Transaction,
		) (TransactionResult, error) {
			return b.computer.ExecuteTransaction(ledger, tx)
		},
	)
	if err != nil {
		// fail fast if fatal error occurs
		return TransactionResult{}, err
	}

	return receipt, nil
}

// CommitBlock seals the current pending block and saves it to storage.
//
// This function clears the pending transaction pool and resets the pending block.
func (b *Blockchain) CommitBlock() (*types.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.commitBlock()
}

func (b *Blockchain) commitBlock() (*types.Block, error) {
	// pending block cannot be committed before execution starts (unless empty)
	if !b.pendingBlock.ExecutionStarted() && !b.pendingBlock.Empty() {
		return nil, &ErrPendingBlockCommitBeforeExecution{BlockHash: b.pendingBlock.Hash()}
	}

	// pending block cannot be committed before execution completes
	if b.pendingBlock.ExecutionStarted() && !b.pendingBlock.ExecutionComplete() {
		return nil, &ErrPendingBlockMidExecution{BlockHash: b.pendingBlock.Hash()}
	}

	block := b.pendingBlock.Block()
	ledger := b.pendingBlock.Ledger()
	events := b.pendingBlock.Events()

	transactions := make([]flow.Transaction, b.pendingBlock.Size())
	for i, tx := range b.pendingBlock.Transactions() {
		// TODO: store reverted status in receipt, seal all transactions
		if tx.Status != flow.TransactionReverted {
			tx.Status = flow.TransactionSealed
		}

		transactions[i] = tx
	}

	// commit the pending block to storage
	err := b.storage.CommitBlock(block, transactions, ledger, events)
	if err != nil {
		return nil, err
	}

	// update system state based on emitted events
	b.handleEvents(events, b.pendingBlock.Number())

	// reset pending block using current block and ledger state
	b.pendingBlock = newPendingBlock(block, ledger)

	return &block, nil
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) ExecuteAndCommitBlock() (*types.Block, []TransactionResult, error) {
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
func (b *Blockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return err
	}

	latestLedger, err := b.storage.LedgerByNumber(latestBlock.Number)
	if err != nil {
		return err
	}

	// reset pending block using latest committed block and ledger state
	b.pendingBlock = newPendingBlock(*latestBlock, latestLedger)

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *Blockchain) ExecuteScript(script []byte) (ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return ScriptResult{}, err
	}

	latestLedger, err := b.storage.LedgerByNumber(latestBlock.Number)
	if err != nil {
		return ScriptResult{}, err
	}

	result, err := b.computer.ExecuteScript(latestLedger.NewView(), script)
	if err != nil {
		return ScriptResult{}, err
	}

	return result, nil
}

// TODO: implement
func (b *Blockchain) ExecuteScriptAtBlock(script []byte, blockNumber uint64) (ScriptResult, error) {
	panic("not implemented")
}

// LastCreatedAccount returns the last account that was created in the blockchain.
func (b *Blockchain) LastCreatedAccount() flow.Account {
	account, _ := b.getAccount(b.lastCreatedAddress)
	return *account
}

// verifySignatures verifies that a transaction contains the necessary signatures.
//
// An error is returned if any of the expected signatures are invalid or missing.
func (b *Blockchain) verifySignatures(tx flow.Transaction) error {
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
func (b *Blockchain) CreateAccount(
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

	err = b.AddTransaction(tx)
	if err != nil {
		return flow.Address{}, err
	}

	_, _, err = b.ExecuteAndCommitBlock()
	if err != nil {
		return flow.Address{}, err
	}

	return b.LastCreatedAccount().Address, nil
}

// UpdateAccountCode submits a transaction to update the code of an existing account
// with the given code. The transaction is paid by the root account.
func (b *Blockchain) UpdateAccountCode(
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

	err = b.AddTransaction(tx)
	if err != nil {
		return err
	}

	_, _, err = b.ExecuteAndCommitBlock()
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
func (b *Blockchain) verifyAccountSignature(
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
func (b *Blockchain) handleEvents(events []flow.Event, blockNumber uint64) {
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

	defaultConfig.RootAccountKey = defaultRootKey
}
