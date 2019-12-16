package emulator

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

// A pendingBlock contains the pending state required to form a new block.
type pendingBlock struct {
	// block information (Number, PreviousBlockHash, TransactionHashes)
	block *types.Block
	// mapping from transaction hash to transaction
	transactions map[string]*flow.Transaction
	// current working ledger, updated after each transaction execution
	ledgerView *types.LedgerView
	// events emitted during execution
	events []flow.Event
	// index of transaction execution
	index int
}

// newPendingBlock creates a new pending block sequentially after a specified block.
func newPendingBlock(prevBlock types.Block, ledgerView *types.LedgerView) *pendingBlock {
	transactions := make(map[string]*flow.Transaction)
	transactionHashes := make([]crypto.Hash, 0)

	block := &types.Block{
		Number:            prevBlock.Number + 1,
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: transactionHashes,
	}

	return &pendingBlock{
		block:        block,
		transactions: transactions,
		ledgerView:   ledgerView,
		events:       make([]flow.Event, 0),
		index:        0,
	}
}

// Hash returns the hash of the pending block.
func (b *pendingBlock) Hash() crypto.Hash {
	return b.block.Hash()
}

// Number returns the number of the pending block.
func (b *pendingBlock) Number() uint64 {
	return b.block.Number
}

// Block returns the block information for the pending block.
func (b *pendingBlock) Block() types.Block {
	return *b.block
}

// LedgerDelta returns the ledger delta for the pending block.
func (b *pendingBlock) LedgerDelta() *types.LedgerDelta {
	return b.ledgerView.Delta()
}

// AddTransaction adds a transaction to the pending block.
func (b *pendingBlock) AddTransaction(tx flow.Transaction) {
	b.block.TransactionHashes = append(b.block.TransactionHashes, tx.Hash())
	b.transactions[string(tx.Hash())] = &tx
}

// ContainsTransaction checks if a transaction is included in the pending block.
func (b *pendingBlock) ContainsTransaction(txHash crypto.Hash) bool {
	_, exists := b.transactions[string(txHash)]
	return exists
}

// GetTransaction retrieves a transaction in the pending block by hash, or nil
// if it does not exist.
func (b *pendingBlock) GetTransaction(txHash crypto.Hash) *flow.Transaction {
	return b.transactions[string(txHash)]
}

// nextTransaction returns the next indexed transaction.
func (b *pendingBlock) nextTransaction() *flow.Transaction {
	txHash := b.block.TransactionHashes[b.index]
	return b.GetTransaction(txHash)
}

// Transactions returns the transactions in the pending block.
func (b *pendingBlock) Transactions() []flow.Transaction {
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
func (b *pendingBlock) ExecuteNextTransaction(
	execute func(ledgerView *types.LedgerView, tx flow.Transaction) (TransactionResult, error),
) (TransactionResult, error) {
	tx := b.nextTransaction()

	result, err := execute(b.ledgerView, *tx)
	if err != nil {
		// fail fast if fatal error occurs
		return TransactionResult{}, err
	}

	// increment transaction index even if transaction reverts
	b.index++

	if result.Reverted() {
		tx.Status = flow.TransactionReverted
	} else {
		tx.Status = flow.TransactionFinalized
		tx.Events = result.Events

		b.events = append(b.events, result.Events...)
	}

	return result, nil
}

// Events returns all events captured during the execution of the pending block.
func (b *pendingBlock) Events() []flow.Event {
	return b.events
}

// ExecutionStarted returns true if the pending block has started executing.
func (b *pendingBlock) ExecutionStarted() bool {
	return b.index > 0
}

// ExecutionComplete returns true if the pending block is fully executed.
func (b *pendingBlock) ExecutionComplete() bool {
	return b.index >= b.Size()
}

// Size returns the number of transactions in the pending block.
func (b *pendingBlock) Size() int {
	return len(b.block.TransactionHashes)
}

// Empty returns true if the pending block is empty.
func (b *pendingBlock) Empty() bool {
	return b.Size() == 0
}
