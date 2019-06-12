package access

import (
	"context"
	"math/big"

	"github.com/dapperlabs/bamboo-emulator/data"
)

// Node simulates the behaviour of a Bamboo access node.
type Node interface {
	Start(context.Context)
	SendTransaction(*data.Transaction) error
	GetBlockByHash(hash data.Hash) (*data.Block, error)
	GetBlockByNumber(number uint64) (*data.Block, error)
	GetLatestBlock() *data.Block
	GetTransaction(hash data.Hash) (*data.Transaction, error)
	GetBalance(address data.Address) (*big.Int, error)
}

type node struct {
	state             data.WorldState
	transactionsIn    chan *data.Transaction
	collectionBuilder *CollectionBuilder
}

// NewNode returns a new simulated access node.
func NewNode(state data.WorldState, collectionsOut chan *data.Collection) Node {
	transactionsIn := make(chan *data.Transaction, 16)

	collectionBuilder := NewCollectionBuilder(transactionsIn, collectionsOut)

	return &node{
		state:             state,
		transactionsIn:    transactionsIn,
		collectionBuilder: collectionBuilder,
	}
}

func (n *node) Start(ctx context.Context) {
	n.collectionBuilder.Start(ctx)
}

func (n *node) SendTransaction(tx *data.Transaction) error {
	err := n.state.InsertTransaction(tx)
	if err != nil {
		return &DuplicateTransactionError{txHash: tx.Hash()}
	}

	n.transactionsIn <- tx

	return nil
}

func (n *node) GetBlockByHash(hash data.Hash) (*data.Block, error) {
	block, err := n.state.GetBlockByHash(hash)
	if err != nil {
		return nil, &BlockNotFoundError{blockHash: &hash}
	}

	return block, nil
}

func (n *node) GetBlockByNumber(number uint64) (*data.Block, error) {
	block, err := n.state.GetBlockByNumber(number)
	if err != nil {
		return nil, &BlockNotFoundError{blockNumber: number}
	}

	return block, nil
}

func (n *node) GetLatestBlock() *data.Block {
	return n.state.GetLatestBlock()
}

func (n *node) GetTransaction(hash data.Hash) (*data.Transaction, error) {
	tx, err := n.state.GetTransaction(hash)
	if err != nil {
		return nil, &TransactionNotFoundError{txHash: hash}
	}

	return tx, nil
}

func (n *node) GetBalance(address data.Address) (*big.Int, error) {
	return nil, nil
}

func (n *node) CallContract(script []byte) (interface{}, error) {
	return nil, nil
}
