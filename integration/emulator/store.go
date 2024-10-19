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

// Package storage defines the interface and implementations for interacting with
// persistent chain state.
package emulator

import (
	"context"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/psiemens/graceland"
)

// EmulatorStorage defines the storage layer for persistent chain state.
//
// This includes finalized blocks and transactions, and the resultant register
// states and emitted events. It does not include pending state, such as pending
// transactions and register states.
//
// Implementations must distinguish between not found errors and errors with
// the underlying storage by returning an instance of store.ErrNotFound if a
// resource cannot be found.
//
// Implementations must be safe for use by multiple goroutines.

type EmulatorStorage interface {
	graceland.Routine
	environment.Blocks
	access.Blocks
	LatestBlockHeight(ctx context.Context) (uint64, error)

	// LatestBlock returns the block with the highest block height.
	LatestBlock(ctx context.Context) (flowgo.Block, error)

	// StoreBlock stores the block in storage. If the exactly same block is already in a storage, return successfully
	StoreBlock(ctx context.Context, block *flowgo.Block) error

	// BlockByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	BlockByID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.Block, error)

	// BlockByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	BlockByHeight(ctx context.Context, height uint64) (*flowgo.Block, error)

	// CommitBlock atomically saves the execution results for a block.
	CommitBlock(
		ctx context.Context,
		block flowgo.Block,
		collections []*flowgo.LightCollection,
		transactions map[flowgo.Identifier]*flowgo.TransactionBody,
		transactionResults map[flowgo.Identifier]*StorableTransactionResult,
		executionSnapshot *snapshot.ExecutionSnapshot,
		events []flowgo.Event,
	) error

	// CollectionByID gets the collection (transaction IDs only) with the given ID.
	CollectionByID(ctx context.Context, collectionID flowgo.Identifier) (flowgo.LightCollection, error)

	// FullCollectionByID gets the full collection (including transaction bodies) with the given ID.
	FullCollectionByID(ctx context.Context, collectionID flowgo.Identifier) (flowgo.Collection, error)

	// TransactionByID gets the transaction with the given ID.
	TransactionByID(ctx context.Context, transactionID flowgo.Identifier) (flowgo.TransactionBody, error)

	// TransactionResultByID gets the transaction result with the given ID.
	TransactionResultByID(ctx context.Context, transactionID flowgo.Identifier) (StorableTransactionResult, error)

	// LedgerByHeight returns a storage snapshot into the ledger state
	// at a given block.
	LedgerByHeight(
		ctx context.Context,
		blockHeight uint64,
	) (snapshot.StorageSnapshot, error)

	// EventsByHeight returns the events in the block at the given height, optionally filtered by type.
	EventsByHeight(ctx context.Context, blockHeight uint64, eventType string) ([]flowgo.Event, error)
}
