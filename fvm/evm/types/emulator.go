package types

import (
	"math/big"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
)

// ReadOnlyBlockView provides a read only view of a block
type ReadOnlyBlockView interface {
	// BalanceOf returns the balance of this address
	BalanceOf(address Address) (*big.Int, error)
	// NonceOf returns the nonce of this address
	NonceOf(address Address) (uint64, error)
	// CodeOf returns the code for this address
	CodeOf(address Address) (Code, error)
	// CodeHashOf returns the code hash for this address
	CodeHashOf(address Address) ([]byte, error)
}

// BlockView facilitates execution of a transaction or a direct evm call in the context of a block
// Any error returned by any of the methods (e.g. stateDB errors) if non-fatal stops the outer flow transaction
// if fatal stops the node.
// EVM validation errors and EVM execution errors are part of the returned result
// and should be handled separately.
type BlockView interface {
	// DirectCall executes a direct call
	DirectCall(call *DirectCall) (*Result, error)

	// RunTransaction executes an evm transaction
	RunTransaction(tx *gethTypes.Transaction) (*Result, error)

	// DryRunTransaction executes unsigned transaction but does not persist the state changes,
	// since transaction is not signed, from address is used as the signer.
	DryRunTransaction(tx *gethTypes.Transaction, from gethCommon.Address) (*Result, error)

	// BatchRunTransactions executes a batch of evm transactions producing
	// a slice of execution Result where each result corresponds to each
	// item in the txs slice.
	BatchRunTransactions(txs []*gethTypes.Transaction) ([]*Result, error)
}

// Emulator emulates an evm-compatible chain
type Emulator interface {
	// constructs a new block view
	NewReadOnlyBlockView(conf *Config) (ReadOnlyBlockView, error)

	// constructs a new block
	NewBlockView(conf *Config) (BlockView, error)
}
