package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A PendingBlock contains the pending state required to form a new block.
type PendingBlock struct {
	// block information (Number, PreviousBlockHash, TransactionHashes)
	block *Block
	// mapping from transaction hash to transaction
	transactions map[string]*flow.Transaction
	// current working ledger, updated after each transaction execution
	ledger flow.Ledger
	// events emitted during execution
	events []flow.Event
	// index of transaction execution
	index int
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
		events:       make([]flow.Event, 0),
		index:        0,
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

// Ledger returns the ledger for the pending block.
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
	txHash := b.block.TransactionHashes[b.index]
	return b.GetTransaction(txHash)
}

// Transactions returns the transactions in the pending block.
func (b *PendingBlock) Transactions() []flow.Transaction {
	transactions := make([]flow.Transaction, len(b.block.TransactionHashes))

	for i, txHash := range b.block.TransactionHashes {
		transactions[i] = *b.transactions[string(txHash)]
	}

	return transactions
}

// ExecuteNextTransaction executes the next transaction in the pending block.
//
// This function uses the provided execute function to perform the actual
// execution, then updates the pending block with the output.
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

			b.events = append(b.events, events...)
			b.ledger.MergeWith(ledger.Updated())
		},
		func() {
			tx.Status = flow.TransactionReverted
		},
	)

	b.index++
}

func (b *PendingBlock) Events() []flow.Event {
	return b.events
}

// ExecutionStarted returns true if the pending block has started executing.
func (b *PendingBlock) ExecutionStarted() bool {
	return b.index > 0
}

// ExecutionComplete returns true if the pending block is fully executed.
func (b *PendingBlock) ExecutionComplete() bool {
	return b.index >= b.Size()
}

// Size returns the number of transactions in the pending block.
func (b *PendingBlock) Size() int {
	return len(b.block.TransactionHashes)
}

// Empty returns true if the pending block is empty.
func (b *PendingBlock) Empty() bool {
	return b.Size() == 0
}
