package access

import (
	"context"
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/data"
)

// Node is a mock implementation of a Bamboo Access Node.
type Node interface {
	Start(context.Context)
	SubmitTransaction(*data.Transaction) error
}

type node struct {
	transactions      chan *data.Transaction
	collectionBuilder *CollectionBuilder
}

// NewNode creates a new mock Access Node.
func NewNode(collectionsOut chan *data.Collection) Node {
	transactions := make(chan *data.Transaction, 16)

	collectionBuilder := NewCollectionBuilder(transactions, collectionsOut)

	return &node{
		transactions:      transactions,
		collectionBuilder: collectionBuilder,
	}
}

func (n *node) Start(ctx context.Context) {
	fmt.Println("Starting access node...")
	n.collectionBuilder.Start(ctx)
}

func (n *node) SubmitTransaction(tx *data.Transaction) error {
	n.transactions <- tx
	return nil
}
