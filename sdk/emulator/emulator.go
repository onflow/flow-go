package emulator

import (
	"sync"
	"time"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/encoding"
	"github.com/dapperlabs/flow-go/pkg/hash"
	"github.com/dapperlabs/flow-go/pkg/language/runtime"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/emulator/state"
	etypes "github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

// EmulatedBlockchain simulates a blockchain for testing purposes.
//
// An emulated blockchain contains a versioned world state store and a pending transaction pool
// for granular state update tests.
//
// The intermediate world state is stored after each transaction is executed and can be used
// to check the output of a single transaction.
//
// The final world state is committed after each block. An index of committed world states is maintained
// to allow a test to seek to a previously committed world state.
type EmulatedBlockchain struct {
	// worldStates is a mapping of committed world states (updated after CommitBlock)
	worldStates map[string][]byte
	// intermediateWorldStates is mapping of intermediate world states (updated after SubmitTransaction)
	intermediateWorldStates map[string][]byte
	// pendingWorldState is the current working world state
	pendingWorldState *state.WorldState
	// txPool is a pool of pending transactions waiting to be committed (already executed)
	txPool             map[string]*types.Transaction
	mut                sync.RWMutex
	computer           *execution.Computer
	rootAccountAddress types.Address
	rootAccountKey     types.AccountPrivateKey
	lastCreatedAccount types.Account
	onEventEmitted     func(event types.Event, blockNumber uint64, txHash crypto.Hash)
}

// EmulatedBlockchainOptions is a set of configuration options for an emulated blockchain.
type EmulatedBlockchainOptions struct {
	RootAccountKey *types.AccountPrivateKey
	OnLogMessage   func(string)
	OnEventEmitted func(event types.Event, blockNumber uint64, txHash crypto.Hash)
}

// DefaultOptions is the default configuration for an emulated blockchain.
var DefaultOptions = EmulatedBlockchainOptions{
	OnLogMessage:   func(string) {},
	OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {},
}

// NewEmulatedBlockchain instantiates a new blockchain backend for testing purposes.
func NewEmulatedBlockchain(opt EmulatedBlockchainOptions) *EmulatedBlockchain {
	// merge user-provided options with default options
	opt = mergeOptions(DefaultOptions, opt)

	worldStates := make(map[string][]byte)
	intermediateWorldStates := make(map[string][]byte)
	txPool := make(map[string]*types.Transaction)
	ws := state.NewWorldState()

	worldStates[string(ws.Hash())] = ws.Encode()

	b := &EmulatedBlockchain{
		worldStates:             worldStates,
		intermediateWorldStates: intermediateWorldStates,
		pendingWorldState:       ws,
		txPool:                  txPool,
		onEventEmitted:          opt.OnEventEmitted,
	}

	runtime := runtime.NewInterpreterRuntime()
	computer := execution.NewComputer(runtime, opt.OnLogMessage)

	b.computer = computer
	rootAccount, rootAccountKey := createRootAccount(ws, opt.RootAccountKey)

	b.rootAccountAddress = rootAccount.Address
	b.rootAccountKey = rootAccountKey
	b.lastCreatedAccount = rootAccount

	return b
}

// RootAccountAddress returns the root account address for this blockchain.
func (b *EmulatedBlockchain) RootAccountAddress() types.Address {
	return b.rootAccountAddress
}

// RootKey returns the root private key for this blockchain.
func (b *EmulatedBlockchain) RootKey() types.AccountPrivateKey {
	return b.rootAccountKey
}

// GetLatestBlock gets the latest sealed block.
func (b *EmulatedBlockchain) GetLatestBlock() *etypes.Block {
	return b.pendingWorldState.GetLatestBlock()
}

// GetBlockByHash gets a block by hash.
func (b *EmulatedBlockchain) GetBlockByHash(hash crypto.Hash) (*etypes.Block, error) {
	block := b.pendingWorldState.GetBlockByHash(hash)
	if block == nil {
		return nil, &ErrBlockNotFound{BlockHash: hash}
	}

	return block, nil
}

// GetBlockByNumber gets a block by number.
func (b *EmulatedBlockchain) GetBlockByNumber(number uint64) (*etypes.Block, error) {
	block := b.pendingWorldState.GetBlockByNumber(number)
	if block == nil {
		return nil, &ErrBlockNotFound{BlockNum: number}
	}

	return block, nil
}

// GetTransaction gets an existing transaction by hash.
//
// First looks in pending txPool, then looks in current blockchain state.
func (b *EmulatedBlockchain) GetTransaction(txHash crypto.Hash) (*types.Transaction, error) {
	b.mut.RLock()
	defer b.mut.RUnlock()

	if tx, ok := b.txPool[string(txHash)]; ok {
		return tx, nil
	}

	tx := b.pendingWorldState.GetTransaction(txHash)
	if tx == nil {
		return nil, &ErrTransactionNotFound{TxHash: txHash}
	}

	return tx, nil
}

// GetTransactionAtVersion gets an existing transaction by hash at a specified state.
func (b *EmulatedBlockchain) GetTransactionAtVersion(txHash, version crypto.Hash) (*types.Transaction, error) {
	ws, err := b.getWorldStateAtVersion(version)
	if err != nil {
		return nil, err
	}

	tx := ws.GetTransaction(txHash)
	if tx == nil {
		return nil, &ErrTransactionNotFound{TxHash: txHash}
	}

	return tx, nil
}

// GetAccount gets account information associated with an address identifier.
func (b *EmulatedBlockchain) GetAccount(address types.Address) (*types.Account, error) {
	registers := b.pendingWorldState.Registers.NewView()
	runtimeContext := execution.NewRuntimeContext(registers)
	account := runtimeContext.GetAccount(address)
	if account == nil {
		return nil, &ErrAccountNotFound{Address: address}
	}

	return account, nil
}

// GetAccountAtVersion gets account information associated with an address identifier at a specified state.
func (b *EmulatedBlockchain) GetAccountAtVersion(address types.Address, version crypto.Hash) (*types.Account, error) {
	ws, err := b.getWorldStateAtVersion(version)
	if err != nil {
		return nil, err
	}

	registers := ws.Registers.NewView()
	runtimeContext := execution.NewRuntimeContext(registers)
	account := runtimeContext.GetAccount(address)
	if account == nil {
		return nil, &ErrAccountNotFound{Address: address}
	}

	return account, nil
}

// SubmitTransaction sends a transaction to the network that is immediately executed (updates blockchain state).
//
// Note that the resulting state is not finalized until CommitBlock() is called.
// However, the pending blockchain state is indexed for testing purposes.
func (b *EmulatedBlockchain) SubmitTransaction(tx types.Transaction) error {
	b.mut.Lock()
	defer b.mut.Unlock()

	hash.SetTransactionHash(&tx)

	// TODO: add more invalid transaction checks
	missingFields := tx.MissingFields()
	if len(missingFields) > 0 {
		return &ErrInvalidTransaction{TxHash: tx.Hash, MissingFields: missingFields}
	}

	if _, exists := b.txPool[string(tx.Hash)]; exists {
		return &ErrDuplicateTransaction{TxHash: tx.Hash}
	}

	if b.pendingWorldState.ContainsTransaction(tx.Hash) {
		return &ErrDuplicateTransaction{TxHash: tx.Hash}
	}

	if err := b.verifySignatures(tx); err != nil {
		return err
	}

	b.txPool[string(tx.Hash)] = &tx
	b.pendingWorldState.InsertTransaction(&tx)

	registers := b.pendingWorldState.Registers.NewView()

	events, err := b.computer.ExecuteTransaction(registers, tx)
	if err != nil {
		b.pendingWorldState.UpdateTransactionStatus(tx.Hash, types.TransactionReverted)

		b.updatePendingWorldStates(tx.Hash)

		return &ErrTransactionReverted{TxHash: tx.Hash, Err: err}
	}

	b.pendingWorldState.SetRegisters(registers.UpdatedRegisters())
	b.pendingWorldState.UpdateTransactionStatus(tx.Hash, types.TransactionFinalized)

	b.updatePendingWorldStates(tx.Hash)

	// TODO: improve the pending block, provide all block information
	prevBlock := b.pendingWorldState.GetLatestBlock()
	blockNumber := prevBlock.Number + 1

	b.emitTransactionEvents(events, blockNumber, tx.Hash)

	return nil
}

func (b *EmulatedBlockchain) updatePendingWorldStates(txHash crypto.Hash) {
	if _, exists := b.intermediateWorldStates[string(txHash)]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.intermediateWorldStates[string(txHash)] = bytes
}

// CallScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) CallScript(script []byte) (interface{}, error) {
	registers := b.pendingWorldState.Registers.NewView()
	value, events, err := b.computer.ExecuteScript(registers, script)
	if err != nil {
		return nil, err
	}

	b.emitScriptEvents(events)

	return value, nil
}

// CallScriptAtVersion executes a read-only script against a specified world state and returns the result.
func (b *EmulatedBlockchain) CallScriptAtVersion(script []byte, version crypto.Hash) (interface{}, error) {
	ws, err := b.getWorldStateAtVersion(version)
	if err != nil {
		return nil, err
	}

	registers := ws.Registers.NewView()

	value, events, err := b.computer.ExecuteScript(registers, script)
	if err != nil {
		return nil, err
	}

	b.emitScriptEvents(events)

	return value, nil
}

// CommitBlock takes all pending transactions and commits them into a block.
//
// Note: this clears the pending transaction pool and indexes the committed blockchain
// state for testing purposes.
func (b *EmulatedBlockchain) CommitBlock() *etypes.Block {
	b.mut.Lock()
	defer b.mut.Unlock()

	txHashes := make([]crypto.Hash, 0)
	for _, tx := range b.txPool {
		txHashes = append(txHashes, tx.Hash)
		if b.pendingWorldState.GetTransaction(tx.Hash).Status != types.TransactionReverted {
			b.pendingWorldState.UpdateTransactionStatus(tx.Hash, types.TransactionSealed)
		}
	}
	b.txPool = make(map[string]*types.Transaction)

	prevBlock := b.pendingWorldState.GetLatestBlock()
	block := &etypes.Block{
		Number:            prevBlock.Number + 1,
		Timestamp:         time.Now(),
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: txHashes,
	}

	b.pendingWorldState.InsertBlock(block)
	b.commitWorldState(block.Hash())

	return block
}

// SeekToState rewinds the blockchain state to a previously committed history.
//
// Note that this only seeks to a committed world state (not intermediate world state)
// and this clears all pending transactions in txPool.
func (b *EmulatedBlockchain) SeekToState(hash crypto.Hash) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if bytes, ok := b.worldStates[string(hash)]; ok {
		ws := state.Decode(bytes)
		b.pendingWorldState = ws
		b.txPool = make(map[string]*types.Transaction)
	}
}

func (b *EmulatedBlockchain) getWorldStateAtVersion(wsHash crypto.Hash) (*state.WorldState, error) {
	if wsBytes, ok := b.worldStates[string(wsHash)]; ok {
		return state.Decode(wsBytes), nil
	}

	if wsBytes, ok := b.intermediateWorldStates[string(wsHash)]; ok {
		return state.Decode(wsBytes), nil
	}

	return nil, &ErrInvalidStateVersion{Version: wsHash}
}

func (b *EmulatedBlockchain) commitWorldState(blockHash crypto.Hash) {
	if _, exists := b.worldStates[string(blockHash)]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.worldStates[string(blockHash)] = bytes
}

// LastCreatedAccount returns the last account that was created in the blockchain.
func (b *EmulatedBlockchain) LastCreatedAccount() types.Account {
	return b.lastCreatedAccount
}

// verifySignatures verifies that a transaction contains the necessary signatures.
//
// An error is returned if any of the expected signatures are invalid or missing.
func (b *EmulatedBlockchain) verifySignatures(tx types.Transaction) error {
	accountWeights := make(map[types.Address]int)

	encodedTx, err := encoding.DefaultEncoder.EncodeCanonicalTransaction(tx)
	if err != nil {
		return err
	}

	for _, accountSig := range tx.Signatures {
		accountPublicKey, err := b.verifyAccountSignature(accountSig, encodedTx)
		if err != nil {
			return err
		}

		accountWeights[accountSig.Account] += accountPublicKey.Weight
	}

	if accountWeights[tx.PayerAccount] < constants.AccountKeyWeightThreshold {
		return &ErrMissingSignature{tx.PayerAccount}
	}

	for _, account := range tx.ScriptAccounts {
		if accountWeights[account] < constants.AccountKeyWeightThreshold {
			return &ErrMissingSignature{account}
		}
	}

	return nil
}

// CreateAccount submits a transaction to create a new account with the given
// account keys and code. The transaction is paid by the root account.
func (b *EmulatedBlockchain) CreateAccount(publicKeys []types.AccountPublicKey, code []byte, nonce uint64) (types.Address, error) {
	createAccountScript, err := templates.CreateAccount(publicKeys, code)
	if err != nil {
		return types.Address{}, nil
	}

	tx := types.Transaction{
		Script:             createAccountScript,
		ReferenceBlockHash: nil,
		Nonce:              nonce,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	if err != nil {
		return types.Address{}, nil
	}

	tx.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx)
	if err != nil {
		return types.Address{}, err
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
	accountSig types.AccountSignature,
	message []byte,
) (accountPublicKey types.AccountPublicKey, err error) {
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

// emitTransactionEvents emits events that occurred during a transaction execution.
//
// This function parses AccountCreated events to update the lastCreatedAccount field.
func (b *EmulatedBlockchain) emitTransactionEvents(events []types.Event, blockNumber uint64, txHash crypto.Hash) {
	for _, event := range events {
		// update lastCreatedAccount if this is an AccountCreated event
		if event.ID == constants.EventAccountCreated {
			accountAddress := event.Values["address"].(types.Address)

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
func (b *EmulatedBlockchain) emitScriptEvents(events []types.Event) {
	for _, event := range events {
		b.onEventEmitted(event, 0, nil)
	}
}

// createRootAccount creates a new root account and commits it to the world state.
func createRootAccount(
	ws *state.WorldState,
	customPrivateKey *types.AccountPrivateKey,
) (types.Account, types.AccountPrivateKey) {
	registers := ws.Registers.NewView()

	var privateKey types.AccountPrivateKey

	if customPrivateKey == nil {
		privateKey, _ = keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
	} else {
		privateKey = *customPrivateKey
	}

	publicKey := privateKey.PublicKey(constants.AccountKeyWeightThreshold)
	publicKeyBytes, _ := types.EncodeAccountPublicKey(publicKey)

	runtimeContext := execution.NewRuntimeContext(registers)
	accountAddress, _ := runtimeContext.CreateAccount(
		[][]byte{publicKeyBytes},
		[]byte{},
	)

	ws.SetRegisters(registers.UpdatedRegisters())

	account := runtimeContext.GetAccount(accountAddress)

	return *account, privateKey
}

// mergeOptions merges the values of two EmulatedBlockchainOptions structs.
// TODO: this will be removed after improvements are made to the way EmulatedBlockchain is configured
func mergeOptions(optA, optB EmulatedBlockchainOptions) EmulatedBlockchainOptions {
	if optB.OnLogMessage == nil {
		optB.OnLogMessage = optA.OnLogMessage
	}

	if optB.OnEventEmitted == nil {
		optB.OnEventEmitted = optA.OnEventEmitted
	}

	return optB
}
