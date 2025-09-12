package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// CollectionGenerator generates collections with consistent randomness.
type CollectionGenerator struct {
	transactionGen *TransactionGenerator
}

func NewCollectionGenerator(
	transactionGen *TransactionGenerator,
) *CollectionGenerator {
	return &CollectionGenerator{
		transactionGen: transactionGen,
	}
}

// WithTxCount is an option that sets the number of transactions in the collection.
func (g *CollectionGenerator) WithTxCount(count int) func(*flow.Collection) {
	return func(collection *flow.Collection) {
		collection.Transactions = g.transactionGen.List(count)
	}
}

// WithTransactions is an option that sets the transactions for the collection.
func (g *CollectionGenerator) WithTransactions(transactions []*flow.TransactionBody) func(*flow.Collection) {
	return func(collection *flow.Collection) {
		collection.Transactions = transactions
	}
}

// Fixture generates a [flow.Collection] with random data based on the provided options.
func (g *CollectionGenerator) Fixture(opts ...func(*flow.Collection)) *flow.Collection {
	collection := &flow.Collection{}

	for _, opt := range opts {
		opt(collection)
	}

	if len(collection.Transactions) == 0 {
		collection.Transactions = g.transactionGen.List(1)
	}

	return collection
}

// List generates a list of [flow.Collection].
func (g *CollectionGenerator) List(n int, opts ...func(*flow.Collection)) []*flow.Collection {
	list := make([]*flow.Collection, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
