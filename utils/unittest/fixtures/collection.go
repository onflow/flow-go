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
	count        int
	transactions []*flow.TransactionBody
}

// WithTxCount returns an option to set the number of transactions in the collection.
func (g *CollectionGenerator) WithTxCount(count int) func(*collectionConfig) {
	return func(config *collectionConfig) {
		config.count = count
	}
}

// WithTransactions returns an option to set the transactions for the collection.
func (g *CollectionGenerator) WithTransactions(transactions []*flow.TransactionBody) func(*collectionConfig) {
	return func(config *collectionConfig) {
		config.transactions = transactions
	}
}

// Fixture generates a collection with optional configuration.
func (g *CollectionGenerator) Fixture(t testing.TB, opts ...func(*collectionConfig)) *flow.Collection {
	config := &collectionConfig{
		count: 1,
	}

	for _, opt := range opts {
		opt(config)
	}

	if len(config.transactions) == 0 {
		config.transactions = g.transactionGen.List(t, config.count)
	}

	return &flow.Collection{Transactions: config.transactions}
}

// List generates a list of collections.
func (g *CollectionGenerator) List(t testing.TB, n int, opts ...func(*collectionConfig)) []*flow.Collection {
	list := make([]*flow.Collection, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
