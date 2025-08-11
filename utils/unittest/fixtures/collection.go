package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

// CollectionGenerator generates collections with consistent randomness.
type CollectionGenerator struct {
	transactionGen *TransactionGenerator
}

// collectionConfig holds the configuration for collection generation.
type collectionConfig struct {
	transactions []*flow.TransactionBody
}

// WithTransactions returns an option to set the transactions for the collection.
func (g *CollectionGenerator) WithTransactions(transactions []*flow.TransactionBody) func(*collectionConfig) {
	return func(config *collectionConfig) {
		config.transactions = transactions
	}
}

// Fixture generates a collection with optional configuration.
func (g *CollectionGenerator) Fixture(t testing.TB, n int, opts ...func(*collectionConfig)) *flow.Collection {
	config := &collectionConfig{}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	if len(config.transactions) == 0 {
		txs := g.transactionGen.List(t, n)
		config.transactions = make([]*flow.TransactionBody, len(txs))
		for i := range txs {
			config.transactions[i] = &txs[i]
		}
	}

	return &flow.Collection{Transactions: config.transactions}
}

// List generates a list of collections.
func (g *CollectionGenerator) List(t testing.TB, n int, txCount int, opts ...func(*collectionConfig)) []*flow.Collection {
	list := make([]*flow.Collection, n)
	for i := range n {
		list[i] = g.Fixture(t, txCount, opts...)
	}
	return list
}
