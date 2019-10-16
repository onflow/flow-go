package storage

import (
	"context"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator/state"
)

// Store defines the DAL layer for the emulator.
type Store interface {
	// Pending world states
	//
	// The pending world state is the current working world state.
	SetPendingWorldState(context.Context, *state.WorldState) error
	GetPendingWorldState(context.Context) (*state.WorldState, error)

	// Committed world states
	//
	// World states are committed when a block is committed.
	CommitWorldState(context.Context, crypto.Hash, *state.WorldState) error
	GetWorldStateByBlockHash(context.Context, crypto.Hash) (*state.WorldState, error)

	// Intermediate world states
	//
	// Intermediate world states represent world states after a transaction
	// has been executed, rather than after a block has been committed.
	AddIntermediateWorldState(context.Context, crypto.Hash, *state.WorldState) error
	GetIntermediateWorldStateByTxHash(context.Context, crypto.Hash) (*state.WorldState, error)

	// Transaction pool
	//
	// The transaction pool stores transactions that have been executed but
	// not committed.
	AddPendingTransaction(context.Context, crypto.Hash, *types.Transaction) error
	GetPendingTransactions(context.Context) ([]*types.Transaction, error)
	ClearPendingTransactions(context.Context) error

	// Root account
	//
	// The root account is the account created when the emulated blockchain
	// is created.
	SetRootAccount(context.Context, types.Address, crypto.PrivateKey) error
	GetRootAccount(context.Context) (types.Address, crypto.PrivateKey, error)
}
