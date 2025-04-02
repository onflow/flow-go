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

package emulator

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
)

// Store implements the Store interface with an in-memory store.
type Store struct {
	mu sync.RWMutex
	// block ID to block height
	blockIDToHeight map[flowgo.Identifier]uint64
	// blocks by height
	blocks map[uint64]flowgo.Block
	// collections by ID
	collections map[flowgo.Identifier]flowgo.LightCollection
	// transactions by ID
	transactions map[flowgo.Identifier]flowgo.TransactionBody
	// Transaction results by ID
	transactionResults map[flowgo.Identifier]StorableTransactionResult
	// Ledger states by block height
	ledger map[uint64]snapshot.SnapshotTree
	// events by block height
	eventsByBlockHeight map[uint64][]flowgo.Event
	// highest block height
	blockHeight uint64
}

var _ environment.Blocks = &Store{}
var _ validator.Blocks = &Store{}
var _ EmulatorStorage = &Store{}

func (b *Store) HeaderByID(id flowgo.Identifier) (*flowgo.Header, error) {
	block, err := b.BlockByID(context.Background(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return block.Header, nil
}

func (b *Store) FinalizedHeader() (*flowgo.Header, error) {
	block, err := b.LatestBlock(context.Background())
	if err != nil {
		return nil, err
	}

	return block.Header, nil
}

func (b *Store) SealedHeader() (*flowgo.Header, error) {
	block, err := b.LatestBlock(context.Background())
	if err != nil {
		return nil, err
	}

	return block.Header, nil
}

func (b *Store) IndexedHeight() (uint64, error) {
	block, err := b.LatestBlock(context.Background())
	if err != nil {
		return 0, err
	}

	return block.Header.Height, nil
}

// ByHeightFrom We don't have to do anything complex here, as emulator does not fork the chain
func (b *Store) ByHeightFrom(height uint64, header *flowgo.Header) (*flowgo.Header, error) {
	if height > header.Height {
		return nil, ErrNotFound
	}
	block, err := b.BlockByHeight(context.Background(), height)
	if err != nil {
		return nil, err
	}

	return block.Header, nil
}

// New returns a new in-memory Store implementation.
func NewMemoryStore() *Store {
	return &Store{
		mu:                  sync.RWMutex{},
		blockIDToHeight:     make(map[flowgo.Identifier]uint64),
		blocks:              make(map[uint64]flowgo.Block),
		collections:         make(map[flowgo.Identifier]flowgo.LightCollection),
		transactions:        make(map[flowgo.Identifier]flowgo.TransactionBody),
		transactionResults:  make(map[flowgo.Identifier]StorableTransactionResult),
		ledger:              make(map[uint64]snapshot.SnapshotTree),
		eventsByBlockHeight: make(map[uint64][]flowgo.Event),
	}
}

func (b *Store) Start() error {
	return nil
}

func (b *Store) Stop() {
}

func (b *Store) LatestBlockHeight(ctx context.Context) (uint64, error) {
	block, err := b.LatestBlock(ctx)
	if err != nil {
		return 0, err
	}

	return block.Header.Height, nil
}

func (b *Store) LatestBlock(_ context.Context) (flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	latestBlock, ok := b.blocks[b.blockHeight]
	if !ok {
		return flowgo.Block{}, ErrNotFound
	}
	return latestBlock, nil
}

func (b *Store) StoreBlock(_ context.Context, block *flowgo.Block) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.storeBlock(block)
}

func (b *Store) storeBlock(block *flowgo.Block) error {
	b.blocks[block.Header.Height] = *block
	b.blockIDToHeight[block.ID()] = block.Header.Height

	if block.Header.Height > b.blockHeight {
		b.blockHeight = block.Header.Height
	}

	return nil
}

func (b *Store) BlockByID(_ context.Context, blockID flowgo.Identifier) (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	blockHeight, ok := b.blockIDToHeight[blockID]
	if !ok {
		return nil, ErrNotFound
	}

	block, ok := b.blocks[blockHeight]
	if !ok {
		return nil, ErrNotFound
	}

	return &block, nil

}

func (b *Store) BlockByHeight(_ context.Context, height uint64) (*flowgo.Block, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, ok := b.blocks[height]
	if !ok {
		return nil, ErrNotFound
	}

	return &block, nil
}

func (b *Store) CommitBlock(
	_ context.Context,
	block flowgo.Block,
	collections []*flowgo.LightCollection,
	transactions map[flowgo.Identifier]*flowgo.TransactionBody,
	transactionResults map[flowgo.Identifier]*StorableTransactionResult,
	executionSnapshot *snapshot.ExecutionSnapshot,
	events []flowgo.Event,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(transactions) != len(transactionResults) {
		return fmt.Errorf(
			"transactions count (%d) does not match result count (%d)",
			len(transactions),
			len(transactionResults),
		)
	}

	err := b.storeBlock(&block)
	if err != nil {
		return err
	}

	for _, col := range collections {
		err := b.InsertCollection(*col)
		if err != nil {
			return err
		}
	}

	for _, tx := range transactions {
		err := b.InsertTransaction(tx.ID(), *tx)
		if err != nil {
			return err
		}
	}

	for txID, result := range transactionResults {
		err := b.InsertTransactionResult(txID, *result)
		if err != nil {
			return err
		}
	}

	err = b.InsertExecutionSnapshot(
		block.Header.Height,
		executionSnapshot)
	if err != nil {
		return err
	}

	err = b.InsertEvents(block.Header.Height, events)
	if err != nil {
		return err
	}

	return nil
}

func (b *Store) CollectionByID(
	_ context.Context,
	collectionID flowgo.Identifier,
) (flowgo.LightCollection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	tx, ok := b.collections[collectionID]
	if !ok {
		return flowgo.LightCollection{}, ErrNotFound
	}
	return tx, nil
}

func (b *Store) FullCollectionByID(
	_ context.Context,
	collectionID flowgo.Identifier,
) (flowgo.Collection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	light, ok := b.collections[collectionID]
	if !ok {
		return flowgo.Collection{}, ErrNotFound
	}

	txs := make([]*flowgo.TransactionBody, len(light.Transactions))
	for i, txID := range light.Transactions {
		tx, ok := b.transactions[txID]
		if !ok {
			return flowgo.Collection{}, ErrNotFound
		}
		txs[i] = &tx
	}

	return flowgo.Collection{
		Transactions: txs,
	}, nil
}

func (b *Store) TransactionByID(
	_ context.Context,
	transactionID flowgo.Identifier,
) (flowgo.TransactionBody, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	tx, ok := b.transactions[transactionID]
	if !ok {
		return flowgo.TransactionBody{}, ErrNotFound
	}
	return tx, nil

}

func (b *Store) TransactionResultByID(
	_ context.Context,
	transactionID flowgo.Identifier,
) (StorableTransactionResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result, ok := b.transactionResults[transactionID]
	if !ok {
		return StorableTransactionResult{}, ErrNotFound
	}
	return result, nil

}

func (b *Store) LedgerByHeight(
	_ context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	return b.ledger[blockHeight], nil
}

func (b *Store) EventsByHeight(
	_ context.Context,
	blockHeight uint64,
	eventType string,
) ([]flowgo.Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	allEvents := b.eventsByBlockHeight[blockHeight]

	events := make([]flowgo.Event, 0)

	for _, event := range allEvents {
		if eventType == "" {
			events = append(events, event)
		} else {
			if string(event.Type) == eventType {
				events = append(events, event)
			}
		}
	}

	return events, nil
}

func (b *Store) InsertCollection(col flowgo.LightCollection) error {
	b.collections[col.ID()] = col
	return nil
}

func (b *Store) InsertTransaction(txID flowgo.Identifier, tx flowgo.TransactionBody) error {
	b.transactions[txID] = tx
	return nil
}

func (b *Store) InsertTransactionResult(txID flowgo.Identifier, result StorableTransactionResult) error {
	b.transactionResults[txID] = result
	return nil
}

func (b *Store) InsertExecutionSnapshot(
	blockHeight uint64,
	executionSnapshot *snapshot.ExecutionSnapshot,
) error {
	oldLedger := b.ledger[blockHeight-1]

	b.ledger[blockHeight] = oldLedger.Append(executionSnapshot)

	return nil
}

func (b *Store) InsertEvents(blockHeight uint64, events []flowgo.Event) error {
	if b.eventsByBlockHeight[blockHeight] == nil {
		b.eventsByBlockHeight[blockHeight] = events
	} else {
		b.eventsByBlockHeight[blockHeight] = append(b.eventsByBlockHeight[blockHeight], events...)
	}

	return nil
}
