// Package store defines the interface and implementations for interacting with
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
	GetBlockByHash(crypto.Hash) (types.Block, error)
	GetBlockByNumber(blockNumber uint64) (types.Block, error)
	GetLatestBlock() (types.Block, error)

	InsertBlock(types.Block) error

	GetTransaction(crypto.Hash) (flow.Transaction, error)
	InsertTransaction(flow.Transaction) error

	GetRegistersView(blockNumber uint64) (flow.RegistersView, error)
	SetRegisters(blockNumber uint64, registers flow.Registers) error

	GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error)
	InsertEvents(blockNumber uint64, events ...flow.Event) error
}
