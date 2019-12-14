package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingBlock is a naive data structure used to represent a pending block in the emulator.
type PendingBlock struct {
	// Block information (Number, PreviousBlockHash, Timestamp, TransactionHashes)
	block *Block
	// Mapping from transaction hash to transaction
	transactions map[string]*flow.Transaction
	// The current working register state, up-to-date with all transactions in the TxPool
	ledger flow.Ledger
	events []*flow.Event
	// Index of transaction execution
	Index int
}

// NewPendingBlock creates a new pending block sequentially after a specified block.
func NewPendingBlock(prevBlock Block, ledger flow.Ledger) *PendingBlock {
	transactions := make(map[string]*flow.Transaction)
	transactionHashes := make([]crypto.Hash, 0)

	block := &Block{
		Number:            prevBlock.Number + 1,
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: transactionHashes,
	}

	return &PendingBlock{
		block:        block,
		transactions: transactions,
		ledger:       ledger,
		Index:        0,
	}
}

// Hash returns the hash of the pending block.
func (b *PendingBlock) Hash() crypto.Hash {
	return b.block.Hash()
}

// Number returns the number of the pending block.
func (b *PendingBlock) Number() uint64 {
	return b.block.Number
}

// Block returns the block information for the pending block.
func (b *PendingBlock) Block() Block {
	return *b.block
}

func (b *PendingBlock) Ledger() flow.Ledger {
	return b.ledger
}

// AddTransaction adds a transaction to the pending block.
func (b *PendingBlock) AddTransaction(tx flow.Transaction) {
	b.block.TransactionHashes = append(b.block.TransactionHashes, tx.Hash())
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

// nextTransaction returns the next indexed transaction.
func (b *PendingBlock) nextTransaction() *flow.Transaction {
	txHash := b.block.TransactionHashes[b.Index]
	return b.GetTransaction(txHash)
}

// Transactions returns the transactions in the pending block.
func (b *PendingBlock) Transactions() []*flow.Transaction {
	transactions := make([]*flow.Transaction, len(b.block.TransactionHashes))

	for i, txHash := range b.block.TransactionHashes {
		transactions[i] = b.transactions[string(txHash)]
	}

	return transactions
}

// TransactionCount retrieves the number of transaction in the pending block.
func (b *PendingBlock) TransactionCount() int {
	return len(b.block.TransactionHashes)
}

// ExecuteNextTransaction executes the next transaction in the pending block.
//
// This function uses the provided `execute` function to perform the actual
// execution, then updates the pending block based on the output.
func (b *PendingBlock) ExecuteNextTransaction(
	execute func(
		tx *flow.Transaction,
		ledger *flow.LedgerView,
		success func(events []flow.Event),
		revert func(),
	),
) {
	tx := b.nextTransaction()

	ledger := b.ledger.NewView()

	execute(
		tx,
		ledger,
		func(events []flow.Event) {
			tx.Status = flow.TransactionFinalized
			tx.Events = events

			b.ledger.MergeWith(ledger.Updated())
		},
		func() {
			tx.Status = flow.TransactionReverted
		},
	)

	b.Index++
}

func (b *PendingBlock) Events() []flow.Event {
	events := make([]flow.Event, 0)

	for _, txHash := range b.block.TransactionHashes {
		tx := b.transactions[string(txHash)]

		events = append(events, tx.Events...)
	}

	return events
}
