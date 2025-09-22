package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// Collection is the default options factory for [flow.Collection] generation.
var Collection collectionFactory

type collectionFactory struct{}

type CollectionOption func(*CollectionGenerator, *flow.Collection)

// WithTxCount is an option that sets the number of transactions in the collection.
func (f collectionFactory) WithTxCount(count int) CollectionOption {
	return func(g *CollectionGenerator, collection *flow.Collection) {
		collection.Transactions = g.transactions.List(count)
	}
}

// WithTransactions is an option that sets the transactions for the collection.
func (f collectionFactory) WithTransactions(transactions ...*flow.TransactionBody) CollectionOption {
	return func(g *CollectionGenerator, collection *flow.Collection) {
		collection.Transactions = transactions
	}
}

// CollectionGenerator generates collections with consistent randomness.
type CollectionGenerator struct {
	collectionFactory

	transactions *TransactionGenerator
}

func NewCollectionGenerator(
	transactions *TransactionGenerator,
) *CollectionGenerator {
	return &CollectionGenerator{
		transactions: transactions,
	}
}

// Fixture generates a [flow.Collection] with random data based on the provided options.
func (g *CollectionGenerator) Fixture(opts ...CollectionOption) *flow.Collection {
	collection := &flow.Collection{}

	for _, opt := range opts {
		opt(g, collection)
	}

	if len(collection.Transactions) == 0 {
		collection.Transactions = g.transactions.List(1)
	}

	return collection
}

// List generates a list of [flow.Collection].
func (g *CollectionGenerator) List(n int, opts ...CollectionOption) []*flow.Collection {
	list := make([]*flow.Collection, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
