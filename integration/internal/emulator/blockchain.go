/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package emulator provides an emulated version of the Flow emulator that can be used
// for development purposes.
//
// This package can be used as a library or as a standalone application.
//
// When used as a library, this package provides tools to write programmatic tests for
// Flow applications.
//
// When used as a standalone application, this package implements the Flow Access API
// and is fully-compatible with Flow gRPC client libraries.
package emulator

import (
	"context"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

// systemChunkTransactionTemplate looks for the RandomBeaconHistory
// heartbeat resource on the service account and calls it.
//
//go:embed templates/systemChunkTransactionTemplate.cdc
var systemChunkTransactionTemplate string

var _ Emulator = &Blockchain{}

// New instantiates a new emulated emulator with the provided options.
func New(opts ...Option) (*Blockchain, error) {

	// apply options to the default config
	conf := defaultConfig
	for _, opt := range opts {
		opt(&conf)
	}
	b := &Blockchain{
		storage:         conf.GetStore(),
		broadcaster:     engine.NewBroadcaster(),
		serviceKey:      conf.GetServiceKey(),
		conf:            conf,
		entropyProvider: &blockHashEntropyProvider{},
	}
	return b.ReloadBlockchain()
}

func (b *Blockchain) Now() time.Time {
	if b.clockOverride != nil {
		return b.clockOverride()
	}
	return time.Now().UTC()
}

// Blockchain emulates the functionality of the Flow emulator.
type Blockchain struct {
	// committed chain state: blocks, transactions, registers, events
	storage     EmulatorStorage
	broadcaster *engine.Broadcaster

	// mutex protecting pending block
	mu sync.RWMutex

	// pending block containing block info, register state, pending transactions
	pendingBlock  *pendingBlock
	clockOverride func() time.Time
	// used to execute transactions and scripts
	vm                   *fvm.VirtualMachine
	vmCtx                fvm.Context
	transactionValidator *access.TransactionValidator
	serviceKey           ServiceKey
	conf                 config
	entropyProvider      *blockHashEntropyProvider
}

func (b *Blockchain) Broadcaster() *engine.Broadcaster {
	return b.broadcaster
}

func (b *Blockchain) ReloadBlockchain() (*Blockchain, error) {

	b.vm = fvm.NewVirtualMachine()
	b.vmCtx = fvm.NewContext(
		fvm.WithLogger(b.conf.Logger),
		fvm.WithCadenceLogging(true),
		fvm.WithChain(b.conf.GetChainID().Chain()),
		fvm.WithBlocks(b.storage),
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(!b.conf.ContractRemovalEnabled),
		fvm.WithComputationLimit(b.conf.ScriptGasLimit),
		fvm.WithAccountStorageLimit(b.conf.StorageLimitEnabled),
		fvm.WithTransactionFeesEnabled(b.conf.TransactionFeesEnabled),
		fvm.WithReusableCadenceRuntimePool(
			reusableRuntime.NewReusableCadenceRuntimePool(
				0,
				runtime.Config{}),
		),
		fvm.WithEntropyProvider(b.entropyProvider),
		fvm.WithEVMEnabled(true),
		fvm.WithAuthorizationChecksEnabled(b.conf.TransactionValidationEnabled),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(b.conf.TransactionValidationEnabled),
	)

	latestBlock, latestLedger, err := configureLedger(
		b.conf,
		b.storage,
		b.vm,
		b.vmCtx)
	if err != nil {
		return nil, err
	}

	b.pendingBlock = newPendingBlock(latestBlock, latestLedger, b.Now())
	err = b.configureTransactionValidator()
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (b *Blockchain) EnableAutoMine() {
	b.conf.AutoMine = true
}

func (b *Blockchain) DisableAutoMine() {
	b.conf.AutoMine = false
}

func (b *Blockchain) Ping() error {
	return nil
}

func (b *Blockchain) GetChain() flowgo.Chain {
	return b.vmCtx.Chain
}

func (b *Blockchain) GetNetworkParameters() access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: b.GetChain().ChainID(),
	}
}

// `blockHashEntropyProvider implements `environment.EntropyProvider`
// which provides a source of entropy to fvm context (required for Cadence's randomness),
// by using the latest block hash.
type blockHashEntropyProvider struct {
	LatestBlock flowgo.Identifier
}

func (gen *blockHashEntropyProvider) RandomSource() ([]byte, error) {
	return gen.LatestBlock[:], nil
}

// make sure `blockHashEntropyProvider implements `environment.EntropyProvider`
var _ environment.EntropyProvider = &blockHashEntropyProvider{}

func (b *Blockchain) configureTransactionValidator() error {
	validator, err := access.NewTransactionValidator(
		b.storage,
		b.conf.GetChainID().Chain(),
		metrics.NewNoopCollector(),
		access.TransactionValidationOptions{
			Expiry:                       b.conf.TransactionExpiry,
			ExpiryBuffer:                 0,
			AllowEmptyReferenceBlockID:   b.conf.TransactionExpiry == 0,
			AllowUnknownReferenceBlockID: false,
			MaxGasLimit:                  b.conf.TransactionMaxGasLimit,
			CheckScriptsParse:            true,
			MaxTransactionByteSize:       flowgo.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flowgo.DefaultMaxCollectionByteSize,
			CheckPayerBalanceMode:        access.Disabled,
		},
		nil,
	)
	if err != nil {
		return err
	}
	b.transactionValidator = validator
	return nil
}

func (b *Blockchain) setFVMContextFromHeader(header *flowgo.Header) fvm.Context {
	b.vmCtx = fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithBlockHeader(header),
	)
	return b.vmCtx
}

// ServiceKey returns the service private key for this emulator.
func (b *Blockchain) ServiceKey() ServiceKey {
	serviceAccount, err := b.getAccount(b.serviceKey.Address)
	if err != nil {
		return b.serviceKey
	}

	if len(serviceAccount.Keys) > 0 {
		b.serviceKey.Index = 0
		b.serviceKey.SequenceNumber = serviceAccount.Keys[0].SeqNumber
		b.serviceKey.Weight = serviceAccount.Keys[0].Weight
	}

	return b.serviceKey
}

// PendingBlockID returns the ID of the pending block.
func (b *Blockchain) PendingBlockID() flowgo.Identifier {
	return b.pendingBlock.BlockID
}

// PendingBlockView returns the view of the pending block.
func (b *Blockchain) PendingBlockView() uint64 {
	return b.pendingBlock.view
}

// PendingBlockTimestamp returns the Timestamp of the pending block.
func (b *Blockchain) PendingBlockTimestamp() time.Time {
	return b.pendingBlock.Block().Header.Timestamp
}

// GetLatestBlock gets the latest sealed block.
func (b *Blockchain) GetLatestBlock() (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getLatestBlock()
}

func (b *Blockchain) getLatestBlock() (*flowgo.Block, error) {
	block, err := b.storage.LatestBlock(context.Background())
	if err != nil {
		return nil, err
	}

	return &block, nil
}

// GetBlockByID gets a block by ID.
func (b *Blockchain) GetBlockByID(id flowgo.Identifier) (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getBlockByID(id)
}

func (b *Blockchain) getBlockByID(id flowgo.Identifier) (*flowgo.Block, error) {
	block, err := b.storage.BlockByID(context.Background(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &BlockNotFoundByIDError{ID: id}
		}

		return nil, err
	}

	return block, nil
}

// GetBlockByHeight gets a block by height.
func (b *Blockchain) GetBlockByHeight(height uint64) (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, err := b.getBlockByHeight(height)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) getBlockByHeight(height uint64) (*flowgo.Block, error) {
	block, err := b.storage.BlockByHeight(context.Background(), height)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &BlockNotFoundByHeightError{Height: height}
		}
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) GetCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getCollectionByID(colID)
}

func (b *Blockchain) getCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error) {
	col, err := b.storage.CollectionByID(context.Background(), colID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &CollectionNotFoundError{ID: colID}
		}
		return nil, err
	}

	return &col, nil
}

func (b *Blockchain) GetFullCollectionByID(colID flowgo.Identifier) (*flowgo.Collection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getFullCollectionByID(colID)
}

func (b *Blockchain) getFullCollectionByID(colID flowgo.Identifier) (*flowgo.Collection, error) {
	col, err := b.storage.FullCollectionByID(context.Background(), colID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &CollectionNotFoundError{ID: colID}
		}
		return nil, err
	}

	return &col, nil
}

// GetTransaction gets an existing transaction by ID.
//
// The function first looks in the pending block, then the current emulator state.
func (b *Blockchain) GetTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getTransaction(txID)
}

func (b *Blockchain) getTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error) {
	pendingTx := b.pendingBlock.GetTransaction(txID)
	if pendingTx != nil {
		return pendingTx, nil
	}

	tx, err := b.storage.TransactionByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &TransactionNotFoundError{ID: txID}
		}
		return nil, err
	}

	return &tx, nil
}

func (b *Blockchain) GetTransactionResult(txID flowgo.Identifier) (*access.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.getTransactionResult(txID)
}

func (b *Blockchain) getTransactionResult(txID flowgo.Identifier) (*access.TransactionResult, error) {
	if b.pendingBlock.ContainsTransaction(txID) {
		return &access.TransactionResult{
			Status: flowgo.TransactionStatusPending,
		}, nil
	}

	storedResult, err := b.storage.TransactionResultByID(context.Background(), txID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &access.TransactionResult{
				Status: flowgo.TransactionStatusUnknown,
			}, nil
		}
		return nil, err
	}

	statusCode := 0
	if storedResult.ErrorCode > 0 {
		statusCode = 1
	}
	result := access.TransactionResult{
		Status:        flowgo.TransactionStatusSealed,
		StatusCode:    uint(statusCode),
		ErrorMessage:  storedResult.ErrorMessage,
		Events:        storedResult.Events,
		TransactionID: txID,
		BlockHeight:   storedResult.BlockHeight,
		BlockID:       storedResult.BlockID,
	}

	return &result, nil
}

// GetAccountByIndex returns the account for the given address.
func (b *Blockchain) GetAccountByIndex(index uint) (*flowgo.Account, error) {

	address, err := b.vmCtx.Chain.ChainID().Chain().AddressAtIndex(uint64(index))
	if err != nil {
		return nil, err
	}

	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Header.Height)

}

// GetAccount returns the account for the given address.
func (b *Blockchain) GetAccount(address flowgo.Address) (*flowgo.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getAccount(address)
}

// getAccount returns the account for the given address.
func (b *Blockchain) getAccount(address flowgo.Address) (*flowgo.Account, error) {
	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.getAccountAtBlock(address, latestBlock.Header.Height)
}

// GetAccountAtBlockHeight  returns the account for the given address at specified block height.
func (b *Blockchain) GetAccountAtBlockHeight(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	account, err := b.getAccountAtBlock(address, blockHeight)
	if err != nil {
		return nil, err
	}

	return account, nil
}

// GetAccountAtBlock returns the account for the given address at specified block height.
func (b *Blockchain) getAccountAtBlock(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error) {
	ledger, err := b.storage.LedgerByHeight(context.Background(), blockHeight)
	if err != nil {
		return nil, err
	}

	account, err := fvm.GetAccount(b.vmCtx, address, ledger)
	if fvmerrors.IsAccountNotFoundError(err) {
		return nil, &AccountNotFoundError{Address: address}
	}

	return account, nil
}

func (b *Blockchain) GetEventsForBlockIDs(eventType string, blockIDs []flowgo.Identifier) (result []flowgo.BlockEvents, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, blockID := range blockIDs {
		block, err := b.storage.BlockByID(context.Background(), blockID)
		if err != nil {
			break
		}
		events, err := b.storage.EventsByHeight(context.Background(), block.Header.Height, eventType)
		if err != nil {
			break
		}
		result = append(result, flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Header.Height,
			BlockTimestamp: block.Header.Timestamp,
			Events:         events,
		})
	}

	return result, err
}

func (b *Blockchain) GetEventsForHeightRange(eventType string, startHeight, endHeight uint64) (result []flowgo.BlockEvents, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for blockHeight := startHeight; blockHeight <= endHeight; blockHeight++ {
		block, err := b.storage.BlockByHeight(context.Background(), blockHeight)
		if err != nil {
			break
		}

		events, err := b.storage.EventsByHeight(context.Background(), blockHeight, eventType)
		if err != nil {
			break
		}

		result = append(result, flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Header.Height,
			BlockTimestamp: block.Header.Timestamp,
			Events:         events,
		})
	}

	return result, err
}

// GetEventsByHeight returns the events in the block at the given height, optionally filtered by type.
func (b *Blockchain) GetEventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.storage.EventsByHeight(context.Background(), blockHeight, eventType)
}

// SendTransaction submits a transaction to the network.
func (b *Blockchain) SendTransaction(flowTx *flowgo.TransactionBody) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	err := b.addTransaction(*flowTx)
	if err != nil {
		return err
	}

	if b.conf.AutoMine {
		_, _, err := b.executeAndCommitBlock()
		if err != nil {
			return err
		}
	}

	return nil
}

// AddTransaction validates a transaction and adds it to the current pending block.
func (b *Blockchain) AddTransaction(tx flowgo.TransactionBody) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.addTransaction(tx)
}

func (b *Blockchain) addTransaction(tx flowgo.TransactionBody) error {

	// If index > 0, pending block has begun execution (cannot add more transactions)
	if b.pendingBlock.ExecutionStarted() {
		return &PendingBlockMidExecutionError{BlockID: b.pendingBlock.BlockID}
	}

	if b.pendingBlock.ContainsTransaction(tx.ID()) {
		return &DuplicateTransactionError{TxID: tx.ID()}
	}

	_, err := b.storage.TransactionByID(context.Background(), tx.ID())
	if err == nil {
		// Found the transaction, this is a duplicate
		return &DuplicateTransactionError{TxID: tx.ID()}
	} else if !errors.Is(err, ErrNotFound) {
		// Error in the storage provider
		return fmt.Errorf("failed to check storage for transaction %w", err)
	}

	err = b.transactionValidator.Validate(context.Background(), &tx)
	if err != nil {
		return ConvertAccessError(err)
	}

	// add transaction to pending block
	b.pendingBlock.AddTransaction(tx)

	return nil
}

// ExecuteBlock executes the remaining transactions in pending block.
func (b *Blockchain) ExecuteBlock() ([]*TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeBlock()
}

func (b *Blockchain) executeBlock() ([]*TransactionResult, error) {
	results := make([]*TransactionResult, 0)

	// empty blocks do not require execution, treat as a no-op
	if b.pendingBlock.Empty() {
		return results, nil
	}

	header := b.pendingBlock.Block().Header
	blockContext := b.setFVMContextFromHeader(header)

	// cannot execute a block that has already executed
	if b.pendingBlock.ExecutionComplete() {
		return results, &PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.BlockID,
		}
	}

	// continue executing transactions until execution is complete
	for !b.pendingBlock.ExecutionComplete() {
		result, err := b.executeNextTransaction(blockContext)
		if err != nil {
			return results, err
		}

		results = append(results, result)
	}

	return results, nil
}

// ExecuteNextTransaction executes the next indexed transaction in pending block.
func (b *Blockchain) ExecuteNextTransaction() (*TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	header := b.pendingBlock.Block().Header
	blockContext := b.setFVMContextFromHeader(header)
	return b.executeNextTransaction(blockContext)
}

// executeNextTransaction is a helper function for ExecuteBlock and ExecuteNextTransaction that
// executes the next transaction in the pending block.
func (b *Blockchain) executeNextTransaction(ctx fvm.Context) (*TransactionResult, error) {
	// check if there are remaining txs to be executed
	if b.pendingBlock.ExecutionComplete() {
		return nil, &PendingBlockTransactionsExhaustedError{
			BlockID: b.pendingBlock.BlockID,
		}
	}

	txnBody := b.pendingBlock.NextTransaction()
	txnId := txnBody.ID()

	// use the computer to execute the next transaction
	output, err := b.pendingBlock.ExecuteNextTransaction(b.vm, ctx)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	tr, err := VMTransactionResultToEmulator(txnId, output)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	return tr, nil
}

// CommitBlock seals the current pending block and saves it to storage.
//
// This function clears the pending transaction pool and resets the pending block.
func (b *Blockchain) CommitBlock() (*flowgo.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	block, err := b.commitBlock()
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (b *Blockchain) commitBlock() (*flowgo.Block, error) {
	// pending block cannot be committed before execution starts (unless empty)
	if !b.pendingBlock.ExecutionStarted() && !b.pendingBlock.Empty() {
		return nil, &PendingBlockCommitBeforeExecutionError{BlockID: b.pendingBlock.BlockID}
	}

	// pending block cannot be committed before execution completes
	if b.pendingBlock.ExecutionStarted() && !b.pendingBlock.ExecutionComplete() {
		return nil, &PendingBlockMidExecutionError{BlockID: b.pendingBlock.BlockID}
	}

	block := b.pendingBlock.Block()
	collections := b.pendingBlock.Collections()
	transactions := b.pendingBlock.Transactions()
	transactionResults, err := convertToSealedResults(b.pendingBlock.TransactionResults(), b.pendingBlock.BlockID, b.pendingBlock.height)
	if err != nil {
		return nil, err
	}

	// lastly we execute the system chunk transaction
	err = b.executeSystemChunkTransaction()
	if err != nil {
		return nil, err
	}

	executionSnapshot := b.pendingBlock.Finalize()
	events := b.pendingBlock.Events()

	// commit the pending block to storage
	err = b.storage.CommitBlock(
		context.Background(),
		*block,
		collections,
		transactions,
		transactionResults,
		executionSnapshot,
		events)
	if err != nil {
		return nil, err
	}

	ledger, err := b.storage.LedgerByHeight(
		context.Background(),
		block.Header.Height,
	)
	if err != nil {
		return nil, err
	}

	// notify listeners on new block
	b.broadcaster.Publish()

	// reset pending block using current block and ledger state
	b.pendingBlock = newPendingBlock(block, ledger, b.Now())
	b.entropyProvider.LatestBlock = block.ID()

	return block, nil
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) ExecuteAndCommitBlock() (*flowgo.Block, []*TransactionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.executeAndCommitBlock()
}

// ExecuteAndCommitBlock is a utility that combines ExecuteBlock with CommitBlock.
func (b *Blockchain) executeAndCommitBlock() (*flowgo.Block, []*TransactionResult, error) {

	results, err := b.executeBlock()
	if err != nil {
		return nil, nil, err
	}

	block, err := b.commitBlock()
	if err != nil {
		return nil, results, err
	}

	blockID := block.ID()
	b.conf.ServerLogger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msgf("📦 Block #%d committed", block.Header.Height)

	return block, results, nil
}

// ResetPendingBlock clears the transactions in pending block.
func (b *Blockchain) ResetPendingBlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	latestBlock, err := b.storage.LatestBlock(context.Background())
	if err != nil {
		return err
	}

	latestLedger, err := b.storage.LedgerByHeight(
		context.Background(),
		latestBlock.Header.Height,
	)
	if err != nil {
		return err
	}

	// reset pending block using latest committed block and ledger state
	b.pendingBlock = newPendingBlock(&latestBlock, latestLedger, b.Now())

	return nil
}

// ExecuteScript executes a read-only script against the world state and returns the result.
func (b *Blockchain) ExecuteScript(
	script []byte,
	arguments [][]byte,
) (*ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, err := b.getLatestBlock()
	if err != nil {
		return nil, err
	}

	return b.executeScriptAtBlockID(script, arguments, latestBlock.Header.ID())
}

func (b *Blockchain) ExecuteScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.executeScriptAtBlockID(script, arguments, id)
}

func (b *Blockchain) executeScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*ScriptResult, error) {
	requestedBlock, err := b.storage.BlockByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	requestedLedgerSnapshot, err := b.storage.LedgerByHeight(
		context.Background(),
		requestedBlock.Header.Height,
	)
	if err != nil {
		return nil, err
	}

	blockContext := fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithBlockHeader(requestedBlock.Header),
	)

	scriptProc := fvm.Script(script).WithArguments(arguments...)

	_, output, err := b.vm.Run(
		blockContext,
		scriptProc,
		requestedLedgerSnapshot)
	if err != nil {
		return nil, err
	}

	scriptID := flowgo.MakeIDFromFingerPrint(script)

	var scriptError error = nil
	var convertedValue cadence.Value = nil

	if output.Err == nil {
		convertedValue = output.Value
	} else {
		scriptError = VMErrorToEmulator(output.Err)
	}

	scriptResult := &ScriptResult{
		ScriptID:        scriptID,
		Value:           convertedValue,
		Error:           scriptError,
		Logs:            output.Logs,
		Events:          output.Events,
		ComputationUsed: output.ComputationUsed,
		MemoryEstimate:  output.MemoryEstimate,
	}

	return scriptResult, nil
}

func (b *Blockchain) ExecuteScriptAtBlockHeight(
	script []byte,
	arguments [][]byte,
	blockHeight uint64,
) (*ScriptResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	requestedBlock, err := b.getBlockByHeight(blockHeight)
	if err != nil {
		return nil, err
	}

	return b.executeScriptAtBlockID(script, arguments, requestedBlock.Header.ID())
}

func convertToSealedResults(
	results map[flowgo.Identifier]IndexedTransactionResult,
	blockID flowgo.Identifier,
	blockHeight uint64,
) (map[flowgo.Identifier]*StorableTransactionResult, error) {

	output := make(map[flowgo.Identifier]*StorableTransactionResult)

	for id, result := range results {
		temp, err := ToStorableResult(result.ProcedureOutput, blockID, blockHeight)
		if err != nil {
			return nil, err
		}
		output[id] = &temp
	}

	return output, nil
}

func (b *Blockchain) GetTransactionsByBlockID(blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, err := b.getBlockByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockID, err)
	}

	var transactions []*flowgo.TransactionBody
	for i, guarantee := range block.Payload.Guarantees {
		c, err := b.getCollectionByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get collection [%d] %s: %w", i, guarantee.CollectionID, err)
		}

		for j, txID := range c.Transactions {
			tx, err := b.getTransaction(txID)
			if err != nil {
				return nil, fmt.Errorf("failed to get transaction [%d] %s: %w", j, txID, err)
			}
			transactions = append(transactions, tx)
		}
	}
	return transactions, nil
}

func (b *Blockchain) GetTransactionResultsByBlockID(blockID flowgo.Identifier) ([]*access.TransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, err := b.getBlockByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockID, err)
	}

	var results []*access.TransactionResult
	for i, guarantee := range block.Payload.Guarantees {
		c, err := b.getCollectionByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get collection [%d] %s: %w", i, guarantee.CollectionID, err)
		}

		for j, txID := range c.Transactions {
			result, err := b.getTransactionResult(txID)
			if err != nil {
				return nil, fmt.Errorf("failed to get transaction result [%d] %s: %w", j, txID, err)
			}
			results = append(results, result)
		}
	}
	return results, nil
}

func (b *Blockchain) GetLogs(identifier flowgo.Identifier) ([]string, error) {
	txResult, err := b.storage.TransactionResultByID(context.Background(), identifier)
	if err != nil {
		return nil, err

	}
	return txResult.Logs, nil
}

// SetClock sets the given clock on blockchain's pending block.
func (b *Blockchain) SetClock(clock func() time.Time) {
	b.clockOverride = clock
	b.pendingBlock.SetTimestamp(clock())
}

// NewScriptEnvironment returns an environment.Environment by
// using as a storage snapshot the blockchain's ledger state.
// Useful for tools that use the emulator's blockchain as a library.
func (b *Blockchain) NewScriptEnvironment() environment.Environment {
	return environment.NewScriptEnvironmentFromStorageSnapshot(
		b.vmCtx.EnvironmentParams,
		b.pendingBlock.ledgerState.NewChild(),
	)
}

func (b *Blockchain) systemChunkTransaction() (*flowgo.TransactionBody, error) {
	serviceAddress := b.GetChain().ServiceAddress()

	script := templates.ReplaceAddresses(
		systemChunkTransactionTemplate,
		templates.Environment{
			RandomBeaconHistoryAddress: serviceAddress.Hex(),
		},
	)

	// TODO: move this to `templates.Environment` struct
	script = strings.ReplaceAll(
		script,
		`import EVM from "EVM"`,
		fmt.Sprintf(
			"import EVM from %s",
			serviceAddress.HexWithPrefix(),
		),
	)

	tx := flowgo.NewTransactionBody().
		SetScript([]byte(script)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		AddAuthorizer(serviceAddress).
		SetPayer(serviceAddress).
		SetReferenceBlockID(b.pendingBlock.parentID)

	return tx, nil
}

func (b *Blockchain) executeSystemChunkTransaction() error {
	txn, err := b.systemChunkTransaction()
	if err != nil {
		return err
	}
	ctx := fvm.NewContextFromParent(
		b.vmCtx,
		fvm.WithLogger(zerolog.Nop()),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithRandomSourceHistoryCallAllowed(true),
		fvm.WithBlockHeader(b.pendingBlock.Block().Header),
	)

	executionSnapshot, output, err := b.vm.Run(
		ctx,
		fvm.Transaction(txn, uint32(len(b.pendingBlock.Transactions()))),
		b.pendingBlock.ledgerState,
	)
	if err != nil {
		return err
	}

	if output.Err != nil {
		return output.Err
	}

	b.pendingBlock.events = append(b.pendingBlock.events, output.Events...)

	err = b.pendingBlock.ledgerState.Merge(executionSnapshot)
	if err != nil {
		return err
	}

	return nil
}

func (b *Blockchain) GetRegisterValues(registerIDs flowgo.RegisterIDs, height uint64) (values []flowgo.RegisterValue, err error) {
	ledger, err := b.storage.LedgerByHeight(context.Background(), height)
	if err != nil {
		return nil, err
	}
	for _, registerID := range registerIDs {
		value, err := ledger.Get(registerID)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
