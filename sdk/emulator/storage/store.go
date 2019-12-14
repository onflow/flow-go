// Package storage defines the interface and implementations for interacting with
// persistent chain state.
package storage

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

// Store defines the storage layer for persistent chain state.
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
type Store interface {

	// GetBlockByHash returns the block with the given hash.
	GetBlockByHash(crypto.Hash) (types.Block, error)

	// GetBlockByNumber returns the block with the given number.
	GetBlockByNumber(blockNumber uint64) (types.Block, error)

	// GetLatestBlock returns the block with the highest block number.
	GetLatestBlock() (types.Block, error)

	// InsertBlock inserts a block.
	InsertBlock(types.Block) error

	// GetTransaction gets the transaction with the given hash.
	GetTransaction(crypto.Hash) (flow.Transaction, error)

	// InsertTransaction inserts a transaction.
	InsertTransaction(flow.Transaction) error

	// CommitBlock atomically saves the execution results for a block.
	CommitBlock(
		block types.Block,
		transactions []flow.Transaction,
		ledger flow.Ledger,
		events []flow.Event,
	) error

	// GetLedger returns the ledger state at a given block.
	GetLedger(blockNumber uint64) (flow.Ledger, error)

	// SetLedger updates all registers in the ledger for the given block.
	// Callers should only include registers in the ledger whose value changed
	// in the given block to save space.
	SetLedger(blockNumber uint64, ledger flow.Ledger) error

	// GetEvents returns all events with the given type between startBlock and
	// endBlock (inclusive). If eventType is empty, returns all events in the
	// range, regardless of type.
	GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error)

	// InsertEvents inserts events for a block.
	InsertEvents(blockNumber uint64, events []flow.Event) error
}
