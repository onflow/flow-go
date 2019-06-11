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
	GetLatestBlock() (*data.Block, error)
	GetTransaction(hash data.Hash) (*data.Transaction, error)
	GetBalance(address data.Address) (*big.Int, error)
}

type node struct {
	transactions      chan *data.Transaction
	collectionBuilder *CollectionBuilder
}

// NewNode returns a new simulated access node.
func NewNode(collectionsOut chan *data.Collection) Node {
	transactions := make(chan *data.Transaction, 16)

	collectionBuilder := NewCollectionBuilder(transactions, collectionsOut)

	return &node{
		transactions:      transactions,
		collectionBuilder: collectionBuilder,
	}
}

func (n *node) Start(ctx context.Context) {
	n.collectionBuilder.Start(ctx)
}

func (n *node) SendTransaction(tx *data.Transaction) error {
	n.transactions <- tx
	return nil
}

func (n *node) GetBlockByHash(hash data.Hash) (*data.Block, error) {
	return nil, nil
}

func (n *node) GetBlockByNumber(number uint64) (*data.Block, error) {
	return nil, nil
}

func (n *node) GetLatestBlock() (*data.Block, error) {
	return nil, nil
}

func (n *node) GetTransaction(hash data.Hash) (*data.Transaction, error) {
	return nil, nil
}

func (n *node) GetBalance(address data.Address) (*big.Int, error) {
	return nil, nil
}

func (n *node) CallContract(script []byte) (interface{}, error) {
	return nil, nil
}
