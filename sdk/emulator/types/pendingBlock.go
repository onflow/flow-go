package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingBlock is a naive data structure used to represent a pending block in the emulator.
type PendingBlock struct {
	// Block information (Number, PreviousBlockHash, Timestamp, TransactionHashes)
	Header *Block
	// Mapping from transaction hash to transaction
	transactions map[string]*flow.Transaction
	// The current working register state, up-to-date with all transactions in the TxPool
	State  flow.Ledger
	events []*flow.Event
	// Index of transaction execution
	Index int
}

// NewPendingBlock creates a new pending block sequentially after a specified block.
func NewPendingBlock(prevBlock Block, state flow.Ledger) *PendingBlock {
	transactions := make(map[string]*flow.Transaction)
	transactionHashes := make([]crypto.Hash, 0)

	header := &Block{
		Number:            prevBlock.Number + 1,
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: transactionHashes,
	}

	return &PendingBlock{
		Header:       header,
		transactions: transactions,
		State:        state,
		Index:        0,
	}
}

// Hash returns the hash of this pending block.
func (b *PendingBlock) Hash() crypto.Hash {
	return b.Header.Hash()
}

// AddTransaction adds a transaction to the pending block.
func (b *PendingBlock) AddTransaction(tx flow.Transaction) {
	b.transactions[string(tx.Hash())] = &tx
}

// ContainsTransaction checks if a transaction is included in the pending block.
func (b *PendingBlock) ContainsTransaction(txHash crypto.Hash) bool {
	_, exists := b.transactions[string(txHash)]
	return exists
}

// GetTransaction retrieves a transaction in the pending block by hash, or nil
// if it does not exist.
func (b *PendingBlock) GetTransaction(txHash crypto.Hash) *flow.Transaction {
	return b.transactions[string(txHash)]
}

// GetNextTransaction returns the next indexed transaction.
func (b *PendingBlock) GetNextTransaction() *flow.Transaction {
	txHash := b.Header.TransactionHashes[b.Index]
	return b.GetTransaction(txHash)
}

// Transactions returns the transactions in the pending block.
func (b *PendingBlock) Transactions() []*flow.Transaction {
	transactions := make([]*flow.Transaction, len(b.Header.TransactionHashes))

	for i, txHash := range b.Header.TransactionHashes {
		transactions[i] = b.transactions[string(txHash)]
	}

	return transactions
}

// TransactionCount retrieves the number of transaction in the pending block.
func (b *PendingBlock) TransactionCount() int {
	return len(b.Header.TransactionHashes)
}

func (b *PendingBlock) Events() []*flow.Event {
	return []*flow.Event{}
}
