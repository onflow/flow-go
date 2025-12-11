package testutil

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// ProtocolDataFixture holds data for blocks and their corresponding protocol data objects.
// All data relationships are properly configured.
type ProtocolDataFixture struct {
	Headers      []*flow.Header
	Blocks       []*flow.Block
	Collections  map[flow.Identifier][]*flow.Collection
	Transactions map[flow.Identifier][]*flow.TransactionBody
	Results      []*flow.ExecutionResult
	Receipts     []*flow.ExecutionReceipt

	ExecutionNodes flow.IdentityList
}

func NewProtocolDataFixture() *ProtocolDataFixture {
	return &ProtocolDataFixture{
		Collections:  make(map[flow.Identifier][]*flow.Collection),
		Transactions: make(map[flow.Identifier][]*flow.TransactionBody),
	}
}

// HeaderByBlockID returns the header with the given block ID.
// If the header is not found, t.Error is called.
func (d *ProtocolDataFixture) HeaderByBlockID(t testing.TB, blockID flow.Identifier) *flow.Header {
	for _, header := range d.Headers {
		if header.ID() == blockID {
			return header
		}
	}
	t.Errorf("HeaderByBlockID: header %s not found", blockID)
	return nil
}

// HeaderByHeight returns the header with the given height.
// If the header is not found, t.Error is called.
func (d *ProtocolDataFixture) HeaderByHeight(t testing.TB, height uint64) *flow.Header {
	for _, header := range d.Headers {
		if header.Height == height {
			return header
		}
	}
	t.Errorf("HeaderByHeight: header %d not found", height)
	return nil
}

// BlockByID returns the block with the given ID.
// If the block is not found, t.Error is called.
func (d *ProtocolDataFixture) BlockByID(t testing.TB, blockID flow.Identifier) *flow.Block {
	for _, block := range d.Blocks {
		if block.ID() == blockID {
			return block
		}
	}
	t.Errorf("BlockByID: block %s not found", blockID)
	return nil
}

// BlockByHeight returns the block with the given height.
// If the block is not found, t.Error is called.
func (d *ProtocolDataFixture) BlockByHeight(t testing.TB, height uint64) *flow.Block {
	for _, block := range d.Blocks {
		if block.Height == height {
			return block
		}
	}
	t.Errorf("BlockByHeight: block %d not found", height)
	return nil
}

// CollectionByID returns the collection with the given ID.
// If the collection is not found, t.Error is called.
func (d *ProtocolDataFixture) CollectionByID(t testing.TB, collectionID flow.Identifier) *flow.Collection {
	for _, blockCollections := range d.Collections {
		for _, collection := range blockCollections {
			if collection.ID() == collectionID {
				return collection
			}
		}
	}
	t.Errorf("CollectionByID: collection %s not found", collectionID)
	return nil
}

// TransactionByID returns the transaction with the given ID.
// If the transaction is not found, t.Error is called.
func (d *ProtocolDataFixture) TransactionByID(t testing.TB, transactionID flow.Identifier) *flow.TransactionBody {
	for _, colTxs := range d.Transactions {
		for _, transaction := range colTxs {
			if transaction.ID() == transactionID {
				return transaction
			}
		}
	}
	t.Errorf("TransactionByID: transaction %s not found", transactionID)
	return nil
}

// TransactionsByBlockID returns an ordered list of transactions in the block.
// If the block is not found, t.Error is called.
func (d *ProtocolDataFixture) TransactionsByBlockID(t testing.TB, blockID flow.Identifier) []*flow.TransactionBody {
	// make sure the block existgs
	_ = d.BlockByID(t, blockID)

	transactions := make([]*flow.TransactionBody, 0)
	for _, col := range d.Collections[blockID] {
		transactions = append(transactions, col.Transactions...)
	}
	return transactions
}

// CollectionIDByTransactionID returns the collection ID for the given transaction ID.
// If the collection ID is not found, t.Error is called.
func (d *ProtocolDataFixture) CollectionIDByTransactionID(t testing.TB, txID flow.Identifier) flow.Identifier {
	for collectionID, colTxs := range d.Transactions {
		for _, transaction := range colTxs {
			if transaction.ID() == txID {
				return collectionID
			}
		}
	}
	t.Errorf("CollectionIDByTransactionID: collection for transaction %s not found", txID)
	return flow.ZeroID
}

type ProtocolDataFixtureBuilder struct {
	blocks         int
	txPerCol       int
	colPerBlock    int
	withReceipts   bool
	executionNodes flow.IdentityList
}

func NewProtocolDataFixtureBuilder() *ProtocolDataFixtureBuilder {
	return &ProtocolDataFixtureBuilder{
		blocks:         10,
		txPerCol:       1,
		colPerBlock:    1,
		withReceipts:   true,
		executionNodes: unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution)),
	}
}

func (tb *ProtocolDataFixtureBuilder) Blocks(n int) *ProtocolDataFixtureBuilder {
	tb.blocks = n
	return tb
}

func (tb *ProtocolDataFixtureBuilder) TransactionsPerCollection(n int) *ProtocolDataFixtureBuilder {
	tb.txPerCol = n
	return tb
}

func (tb *ProtocolDataFixtureBuilder) CollectionsPerBlock(n int) *ProtocolDataFixtureBuilder {
	tb.colPerBlock = n
	return tb
}

func (tb *ProtocolDataFixtureBuilder) Receipts() *ProtocolDataFixtureBuilder {
	tb.withReceipts = true
	return tb
}

func (tb *ProtocolDataFixtureBuilder) ExecutionNodes(nodes flow.IdentityList) *ProtocolDataFixtureBuilder {
	tb.executionNodes = nodes
	return tb
}

func (tb *ProtocolDataFixtureBuilder) Build(t testing.TB) *ProtocolDataFixture {
	d := NewProtocolDataFixture()
	d.ExecutionNodes = tb.executionNodes

	d.Blocks, d.Headers, d.Collections, d.Transactions = tb.buildBlocks()

	if tb.withReceipts {
		d.Results, d.Receipts = tb.buildReceipts(d.Blocks, d.ExecutionNodes)
	}

	return d
}

func (tb *ProtocolDataFixtureBuilder) buildBlocks() ([]*flow.Block, []*flow.Header, map[flow.Identifier][]*flow.Collection, map[flow.Identifier][]*flow.TransactionBody) {
	blocks := make([]*flow.Block, tb.blocks)
	headers := make([]*flow.Header, tb.blocks)
	collections := make(map[flow.Identifier][]*flow.Collection, tb.blocks*tb.colPerBlock)
	txs := make(map[flow.Identifier][]*flow.TransactionBody, tb.blocks*tb.colPerBlock*tb.txPerCol)

	parent := unittest.BlockFixture()
	for i := range tb.blocks {
		guarantees := make([]*flow.CollectionGuarantee, tb.colPerBlock)
		blockCollections := make([]*flow.Collection, tb.colPerBlock)
		for j := range tb.colPerBlock {
			colTxs := unittest.TransactionBodyListFixture(tb.txPerCol)
			col := unittest.CompleteCollectionFromTransactions(colTxs)
			guarantees[j] = col.Guarantee

			blockCollections[j] = col.Collection
			txs[col.Guarantee.CollectionID] = colTxs
		}

		block := unittest.BlockFixture(
			unittest.Block.WithParent(parent.ID(), parent.View, parent.Height),
			unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantees...))),
		)

		blocks[i] = block
		headers[i] = block.ToHeader()
		collections[block.ID()] = blockCollections
		parent = block
	}

	return blocks, headers, collections, txs
}

func (tb *ProtocolDataFixtureBuilder) buildReceipts(blocks []*flow.Block, executionNodes flow.IdentityList) ([]*flow.ExecutionResult, []*flow.ExecutionReceipt) {
	results := make([]*flow.ExecutionResult, len(blocks))
	receipts := make([]*flow.ExecutionReceipt, 0, len(blocks)*len(executionNodes))
	for i := range blocks {
		results[i] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks[i]))
		for _, node := range executionNodes {
			receipts = append(receipts, unittest.ExecutionReceiptFixture(
				unittest.WithResult(results[i]),
				unittest.WithExecutorID(node.NodeID),
			))
		}
	}
	return results, receipts
}
