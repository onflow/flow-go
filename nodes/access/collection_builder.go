package access

import (
	"context"
	"fmt"
	"time"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// CollectionBuilder produces collections from incoming transactions.
type CollectionBuilder struct {
	state               *data.WorldState
	transactionsIn      chan *data.Transaction
	collectionsOut      chan *data.Collection
	pendingTransactions []*data.Transaction
}

// NewCollectionBuilder initializes a new CollectionBuilder with the provided channels.
//
// The collection builder pulls transactions from the transactionsIn channel and pushes
// collections to the collectionsOut channel.
func NewCollectionBuilder(state *data.WorldState, transactionsIn chan *data.Transaction, collectionsOut chan *data.Collection) *CollectionBuilder {
	return &CollectionBuilder{
		state:               state,
		transactionsIn:      transactionsIn,
		collectionsOut:      collectionsOut,
		pendingTransactions: make([]*data.Transaction, 0),
	}
}

// Start starts the collection builder worker loop.
func (c *CollectionBuilder) Start(ctx context.Context) {
	tick := time.Tick(time.Second)
	for {
		select {
		case <-tick:
			c.buildCollection()
		case tx := <-c.transactionsIn:
			c.enqueueTransaction(tx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *CollectionBuilder) enqueueTransaction(tx *data.Transaction) {
	c.pendingTransactions = append(c.pendingTransactions, tx)
}

func (c *CollectionBuilder) buildCollection() {
	if len(c.pendingTransactions) == 0 {
		return
	}

	fmt.Printf("Building collection with %d transactions... \n", len(c.pendingTransactions))

	transactionHashes := make([]crypto.Hash, len(c.pendingTransactions))

	for i, tx := range c.pendingTransactions {
		transactionHashes[i] = tx.Hash()
	}

	collection := &data.Collection{
		TransactionHashes: transactionHashes,
	}

	err := c.state.InsertCollection(collection)
	if err != nil {
		return
	}

	c.collectionsOut <- collection

	c.pendingTransactions = make([]*data.Transaction, 0)
}
