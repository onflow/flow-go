package emulator

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dapperlabs/flow-go/sdk/emulator/store"

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
	storage store.Store

	// Mutex protecting pending register state and txPool
	mu sync.RWMutex
	// The current working register state, up-to-date with all transactions
	// in the txPool.
	pendingState flow.Registers
	// Pool of transactions that have been executed, but not finalized
	txPool map[string]*flow.Transaction

	// The runtime context used to execute transactions and scripts
	computer *execution.Computer

	rootAccountAddress flow.Address
	rootAccountKey     flow.AccountPrivateKey
	lastCreatedAccount flow.Account

	// TODO: Remove this
	// intermediateWorldStates is mapping of intermediate world states (updated after SubmitTransaction)
	intermediateWorldStates map[string][]byte

	// TODO: store events in storage
	onEventEmitted func(event flow.Event, blockNumber uint64, txHash crypto.Hash)
}

// Options is a set of configuration options for an emulated blockchain.
type Options struct {
	RootAccountKey *flow.AccountPrivateKey
	OnLogMessage   func(string)
	OnEventEmitted func(event flow.Event, blockNumber uint64, txHash crypto.Hash)
}

// DefaultOptions is the default configuration for an emulated blockchain.
// NOTE: Instantiated in init function
var DefaultOptions Options

// NewEmulatedBlockchain instantiates a new blockchain backend for testing purposes.
func NewEmulatedBlockchain(opts Options) *EmulatedBlockchain {
	// merge user-provided options with default options
	opts = mergeOptions(DefaultOptions, opts)

	storage := store.NewMemStore()
	initialState := make(flow.Registers)
	txPool := make(map[string]*flow.Transaction)

	// create the root account
	rootAccount := createAccount(initialState, *opts.RootAccountKey)

	b := &EmulatedBlockchain{
		storage:            storage,
		pendingState:       initialState,
		txPool:             txPool,
		onEventEmitted:     opts.OnEventEmitted,
		rootAccountAddress: rootAccount.Address,
		rootAccountKey:     *opts.RootAccountKey,
		lastCreatedAccount: rootAccount,
	}

	interpreterRuntime := runtime.NewInterpreterRuntime()
	computer := execution.NewComputer(interpreterRuntime, opts.OnLogMessage)
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
func (b *EmulatedBlockchain) GetLatestBlock() *types.Block {
	block, err := b.storage.GetLatestBlock()
	if err != nil {
		panic(err)
	}
	return &block
}

// GetBlockByHash gets a block by hash.
func (b *EmulatedBlockchain) GetBlockByHash(hash crypto.Hash) (*types.Block, error) {
	block, err := b.storage.GetBlockByHash(hash)
	if err != nil {
		// TODO: consolidate emulator/storage errors
		return nil, &ErrBlockNotFound{BlockHash: hash}
	}

	return &block, nil
}

// GetBlockByNumber gets a block by number.
func (b *EmulatedBlockchain) GetBlockByNumber(number uint64) (*types.Block, error) {
	block, err := b.storage.GetBlockByNumber(number)
	if err != nil {
		// TODO: consolidate emualator/storage errors
		return nil, &ErrBlockNotFound{BlockNum: number}
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
		return nil, &ErrTransactionNotFound{TxHash: txHash}
	}

	return &tx, nil
}

// TODO: remove this
// GetTransactionAtVersion gets an existing transaction by hash at a specified state.
//func (b *EmulatedBlockchain) GetTransactionAtVersion(txHash, version crypto.Hash) (*flow.Transaction, error) {
//	ws, err := b.getWorldStateAtVersion(version)
//	if err != nil {
//		return nil, err
//	}
//
//	tx := ws.GetTransaction(txHash)
//	if tx == nil {
//		return nil, &ErrTransactionNotFound{TxHash: txHash}
//	}
//
//	return tx, nil
//}

// GetAccount gets account information associated with an address identifier.
func (b *EmulatedBlockchain) GetAccount(address flow.Address) (*flow.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	registers := b.pendingState
	runtimeContext := execution.NewRuntimeContext(registers.NewView())
	account := runtimeContext.GetAccount(address)
	if account == nil {
		return nil, &ErrAccountNotFound{Address: address}
	}

	return account, nil
}

// TODO: replace with block height versioning
// GetAccountAtVersion gets account information associated with an address identifier at a specified state.
//func (b *EmulatedBlockchain) GetAccountAtVersion(address flow.Address, version crypto.Hash) (*flow.Account, error) {
//	ws, err := b.getWorldStateAtVersion(version)
//	if err != nil {
//		return nil, err
//	}
//
//	registers := ws.Registers.NewView()
//	runtimeContext := execution.NewRuntimeContext(registers)
//	account := runtimeContext.GetAccount(address)
//	if account == nil {
//		return nil, &ErrAccountNotFound{Address: address}
//	}
//
//	return account, nil
//}

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

	if _, err := b.storage.GetTransaction(tx.Hash()); err != nil {
		if errors.Is(err, store.ErrNotFound{}) {
			return &ErrDuplicateTransaction{TxHash: tx.Hash()}
		} else {
			return fmt.Errorf("Failed to check storage for transaction %w", err)
		}
	}

	if err := b.verifySignatures(tx); err != nil {
		return err
	}

	tx.Status = flow.TransactionPending
	b.txPool[string(tx.Hash())] = &tx

	registers := b.pendingState.NewView()

	events, err := b.computer.ExecuteTransaction(registers, tx)
	if err != nil {
		tx.Status = flow.TransactionReverted
		return &ErrTransactionReverted{TxHash: tx.Hash(), Err: err}
	}

	// Update pending state with registers changed during transaction execution
	b.pendingState.MergeWith(registers.UpdatedRegisters())

	// TODO should transactions be persisted to storage at this point?
	// Update the transaction's status and events
	// NOTE: this updates txPool state because txPool stores pointers
	tx.Status = flow.TransactionFinalized
	tx.Events = events

	// TODO: improve the pending block, provide all block information
	prevBlock, err := b.storage.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("Failed to get latest block: %w", err)
	}
	blockNumber := prevBlock.Number + 1

	// TODO: remove this. Instead we are storing events in storage, they
	// TODO: should be stored there when the block is committed
	b.emitTransactionEvents(events, blockNumber, tx.Hash())

	return nil
}

// TODO: remove this
//func (b *EmulatedBlockchain) updatePendingWorldStates(txHash crypto.Hash) {
//	//if _, exists := b.intermediateWorldStates[string(txHash)]; exists {
//	//	return
//	//}
//	//
//	//bytes := b.pendingWorldState.Encode()
//	//b.intermediateWorldStates[string(txHash)] = bytes
//}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) ExecuteScript(script []byte) (interface{}, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	registers := b.pendingState.NewView()
	value, events, err := b.computer.ExecuteScript(registers, script)
	if err != nil {
		return nil, err
	}

	// TODO: decide how to handle script events
	b.emitScriptEvents(events)

	return value, nil
}

// TODO: remove this -- replace with versioning by block hash
// ExecuteScriptAtVersion executes a read-only script against a specified world state and returns the result.
//func (b *EmulatedBlockchain) ExecuteScriptAtVersion(script []byte, version crypto.Hash) (interface{}, error) {
//	ws, err := b.getWorldStateAtVersion(version)
//	if err != nil {
//		return nil, err
//	}
//
//	registers := ws.Registers.NewView()
//
//	value, events, err := b.computer.ExecuteScript(registers, script)
//	if err != nil {
//		return nil, err
//	}
//
//	b.emitScriptEvents(events)
//
//	return value, nil
//}

// CommitBlock takes all pending transactions and commits them into a block.
//
// Note: this clears the pending transaction pool and indexes the committed blockchain
// state for testing purposes.
func (b *EmulatedBlockchain) CommitBlock() *types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	txHashes := make([]crypto.Hash, 0)
	for _, tx := range b.txPool {
		txHashes = append(txHashes, tx.Hash())
		if tx.Status != flow.TransactionReverted {
			tx.Status = flow.TransactionSealed
		}
	}
	b.txPool = make(map[string]*flow.Transaction)

	prevBlock, err := b.storage.GetLatestBlock()
	if err != nil {
		// TODO: Bubble up error
		panic(err)
	}
	block := &types.Block{
		Number:            prevBlock.Number + 1,
		Timestamp:         time.Now(),
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: txHashes,
	}

	if err := b.storage.InsertBlock(*block); err != nil {
		// TODO: Bubble up error
		panic(err)
	}

	return block
}

// TODO: remove
// SeekToState rewinds the blockchain state to a previously committed history.
//
// Note that this only seeks to a committed world state (not intermediate world state)
// and this clears all pending transactions in txPool.
//func (b *EmulatedBlockchain) SeekToState(hash crypto.Hash) {
//	b.mu.Lock()
//	defer b.mu.Unlock()
//
//	if bytes, ok := b.worldStates[string(hash)]; ok {
//		ws := state.Decode(bytes)
//		b.pendingWorldState = ws
//		b.txPool = make(map[string]*flow.Transaction)
//	}
//}

// TODO: remove
//func (b *EmulatedBlockchain) getWorldStateAtVersion(wsHash crypto.Hash) (*state.WorldState, error) {
//	if wsBytes, ok := b.worldStates[string(wsHash)]; ok {
//		return state.Decode(wsBytes), nil
//	}
//
//	if wsBytes, ok := b.intermediateWorldStates[string(wsHash)]; ok {
//		return state.Decode(wsBytes), nil
//	}
//
//	return nil, &ErrInvalidStateVersion{Version: wsHash}
//}

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

// verifyAccountSignature verifies that an account signature is valid for the account and given message.
//
// If the signature is valid, this function returns the associated account key.
//
// An error is returned if the account does not contain a public key that correctly verifies the signature
// against the given message.
func (b *EmulatedBlockchain) verifyAccountSignature(
	accountSig flow.AccountSignature,
	message []byte,
) (accountPublicKey flow.AccountPublicKey, err error) {
	account, err := b.GetAccount(accountSig.Account)
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

// TODO remove this in favor of storing events in emulator
// emitTransactionEvents emits events that occurred during a transaction execution.
//
// This function parses AccountCreated events to update the lastCreatedAccount field.
func (b *EmulatedBlockchain) emitTransactionEvents(events []flow.Event, blockNumber uint64, txHash crypto.Hash) {
	for _, event := range events {
		// update lastCreatedAccount if this is an AccountCreated event
		if event.Type == flow.EventAccountCreated {
			accountAddress := event.Values["address"].(flow.Address)

			account, err := b.GetAccount(accountAddress)
			if err != nil {
				panic("failed to get newly-created account")
			}

			b.lastCreatedAccount = *account
		}

		b.onEventEmitted(event, blockNumber, txHash)
	}
}

// emitScriptEvents emits events that occurred during a script execution.
func (b *EmulatedBlockchain) emitScriptEvents(events []flow.Event) {
	for _, event := range events {
		b.onEventEmitted(event, 0, nil)
	}
}

// createAccount creates an account with the given private key and injects it
// into the given state, bypassing the need for a transaction.
func createAccount(registers flow.Registers, privateKey flow.AccountPrivateKey) flow.Account {
	publicKey := privateKey.PublicKey(keys.PublicKeyWeightThreshold)
	publicKeyBytes, err := flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	view := registers.NewView()
	runtimeContext := execution.NewRuntimeContext(view)
	accountAddress, err := runtimeContext.CreateAccount(
		[][]byte{publicKeyBytes},
		[]byte{},
	)
	if err != nil {
		panic(err)
	}

	registers.MergeWith(view.UpdatedRegisters())

	account := runtimeContext.GetAccount(accountAddress)
	return *account
}

// mergeOptions merges the values of two Options structs.
// TODO: this will be removed after improvements are made to the way EmulatedBlockchain is configured
func mergeOptions(optA, optB Options) Options {
	if optB.OnLogMessage == nil {
		optB.OnLogMessage = optA.OnLogMessage
	}

	if optB.OnEventEmitted == nil {
		optB.OnEventEmitted = optA.OnEventEmitted
	}

	return optB
}

func init() {
	// Initialize default emulator options
	defaultRootKey, err := keys.GeneratePrivateKey(
		keys.ECDSA_P256_SHA3_256,
		[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
	if err != nil {
		panic("Failed to generate default root key: " + err.Error())
	}

	DefaultOptions.OnLogMessage = func(string) {}
	DefaultOptions.OnEventEmitted = func(event flow.Event, blockNumber uint64, txHash crypto.Hash) {}
	DefaultOptions.RootAccountKey = &defaultRootKey
}
