package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingBlock is a naive data structure used to represent a pending block in the emulator.
type PendingBlock struct {
	// Block information (Number, PreviousBlockHash, Timestamp, TransactionHashes)
	Header *Block
	// Pool of transactions that have been executed, but not finalized
	TxPool map[string]*flow.Transaction
	// The current working register state, up-to-date with all transactions in the TxPool
	State flow.Ledger
	// Index of transaction execution
	Index int
}

// NewPendingBlock creates a new pending block sequentially after a specified block.
func NewPendingBlock(prevBlock Block, state flow.Ledger) *PendingBlock {
	header := &Block{
		Number:            prevBlock.Number + 1,
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: make([]crypto.Hash, 0),
	}

	return &PendingBlock{
		Header: header,
		TxPool: make(map[string]*flow.Transaction),
		State:  state,
		Index:  0,
	}
}

// Hash returns the hash of this pending block.
func (b *PendingBlock) Hash() crypto.Hash {
	return b.Header.Hash()
}

// AddTransaction adds a transaction to the list of TransactionHashes and TxPool.
func (b *PendingBlock) AddTransaction(tx flow.Transaction) {
	b.TxPool[string(tx.Hash())] = &tx
	b.Header.TransactionHashes = append(b.Header.TransactionHashes, tx.Hash())
}

// ContainsTransaction checks if a transaction is included in the pending block.
func (b *PendingBlock) ContainsTransaction(txHash crypto.Hash) bool {
	_, exists := b.TxPool[string(txHash)]
	return exists
}

// GetTransaction retrieves a transaction stored in TxPool (always preceeded by ContainsTransaction).
func (b *PendingBlock) GetTransaction(txHash crypto.Hash) *flow.Transaction {
	return b.TxPool[string(txHash)]
}

// GetNextTransaction retrieves the next indexed transaction.
func (b *PendingBlock) GetNextTransaction() *flow.Transaction {
	txHash := b.Transactions()[b.Index]
	return b.GetTransaction(txHash)
}

// Transactions retrieves the list of transaction hashes in the pending block.
func (b *PendingBlock) Transactions() []crypto.Hash {
	return b.Header.TransactionHashes
}

// TransactionCount retrieves the number of transaction in the pending block.
func (b *PendingBlock) TransactionCount() int {
	return len(b.Header.TransactionHashes)
}
