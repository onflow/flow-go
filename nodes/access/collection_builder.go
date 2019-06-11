package access

import (
	"context"
	"fmt"
	"time"

	"github.com/dapperlabs/bamboo-emulator/data"
)

// CollectionBuilder produces collections from incoming transactions.
type CollectionBuilder struct {
	collections         chan *data.Collection
	transactions        chan *data.Transaction
	pendingTransactions []*data.Transaction
}

// NewCollectionBuilder initializes a new CollectionBuilder with the provided channels.
func NewCollectionBuilder(collections chan *data.Collection, transactions chan *data.Transaction) *CollectionBuilder {
	return &CollectionBuilder{
		collections:         collections,
		transactions:        transactions,
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
		case tx := <-c.transactions:
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

	// TODO: form collection once data type is finished
	collection := &data.Collection{}
	c.pendingTransactions = make([]*data.Transaction, 0)

	c.collections <- collection
}
