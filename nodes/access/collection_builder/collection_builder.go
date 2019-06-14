package collection_builder

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// CollectionBuilder produces collections from incoming transactions.
type CollectionBuilder struct {
	state               *data.WorldState
	transactionsIn      <-chan *data.Transaction
	collectionsOut      chan<- *data.Collection
	pendingTransactions []*data.Transaction
	log                 *logrus.Logger
}

// NewCollectionBuilder initializes a new CollectionBuilder with the provided channels.
//
// The collection builder pulls transactions from the transactionsIn channel and pushes
// collections to the collectionsOut channel.
func NewCollectionBuilder(
	state *data.WorldState,
	transactionsIn <-chan *data.Transaction,
	collectionsOut chan<- *data.Collection,
	log *logrus.Logger,
) *CollectionBuilder {
	return &CollectionBuilder{
		state:               state,
		transactionsIn:      transactionsIn,
		collectionsOut:      collectionsOut,
		pendingTransactions: make([]*data.Transaction, 0),
		log:                 log,
	}
}

// Start starts the collection builder worker loop.
func (c *CollectionBuilder) Start(ctx context.Context, interval time.Duration) {
	tick := time.Tick(interval)
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

	c.log.
		WithFields(logrus.Fields{
			"collectionHash": collection.Hash(),
			"collectionSize": len(c.pendingTransactions),
		}).
		Infof(
			"Publishing collection with %d transaction(s)",
			len(c.pendingTransactions),
		)

	c.collectionsOut <- collection

	c.pendingTransactions = make([]*data.Transaction, 0)
}
