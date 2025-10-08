package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionResult is the default options factory for [flow.TransactionResult] generation.
var TransactionResult transactionResultFactory

type transactionResultFactory struct{}

type TransactionResultOption func(*TransactionResultGenerator, *flow.TransactionResult)

// WithTransactionID is an option that sets the transaction ID for the transaction result.
func (f transactionResultFactory) WithTransactionID(transactionID flow.Identifier) TransactionResultOption {
	return func(g *TransactionResultGenerator, result *flow.TransactionResult) {
		result.TransactionID = transactionID
	}
}

// WithErrorMessage is an option that sets the error message for the transaction result.
func (f transactionResultFactory) WithErrorMessage(errorMessage string) TransactionResultOption {
	return func(g *TransactionResultGenerator, result *flow.TransactionResult) {
		result.ErrorMessage = errorMessage
	}
}

// WithComputationUsed is an option that sets the computation used for the transaction result.
func (f transactionResultFactory) WithComputationUsed(computationUsed uint64) TransactionResultOption {
	return func(g *TransactionResultGenerator, result *flow.TransactionResult) {
		result.ComputationUsed = computationUsed
	}
}

// TransactionResultGenerator generates transaction results with consistent randomness.
type TransactionResultGenerator struct {
	transactionResultFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
}

func NewTransactionResultGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
) *TransactionResultGenerator {
	return &TransactionResultGenerator{
		random:      random,
		identifiers: identifiers,
	}
}

// Fixture generates a [flow.TransactionResult] with random data based on the provided options.
func (g *TransactionResultGenerator) Fixture(opts ...TransactionResultOption) flow.TransactionResult {
	result := flow.TransactionResult{
		TransactionID:   g.identifiers.Fixture(),
		ComputationUsed: g.random.Uint64InRange(1, 9999),
		ErrorMessage:    "",
	}

	for _, opt := range opts {
		opt(g, &result)
	}

	return result
}

// List generates a list of [flow.TransactionResult].
func (g *TransactionResultGenerator) List(n int, opts ...TransactionResultOption) []flow.TransactionResult {
	list := make([]flow.TransactionResult, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// ForTransactions generates a list of [flow.TransactionResult] for multiple transactions.
func (g *TransactionResultGenerator) ForTransactions(transactions []*flow.TransactionBody, opts ...TransactionResultOption) []flow.TransactionResult {
	list := make([]flow.TransactionResult, len(transactions))
	for i, tx := range transactions {
		nOpts := append(opts, TransactionResult.WithTransactionID(tx.ID()))
		list[i] = g.Fixture(nOpts...)
	}
	return list
}

var LightTransactionResult lightTransactionResultFactory

type lightTransactionResultFactory struct{}

type LightTransactionResultOption func(*LightTransactionResultGenerator, *flow.LightTransactionResult)

// WithTransactionID is an option that sets the transaction ID for the transaction result.
func (f lightTransactionResultFactory) WithTransactionID(transactionID flow.Identifier) LightTransactionResultOption {
	return func(g *LightTransactionResultGenerator, result *flow.LightTransactionResult) {
		result.TransactionID = transactionID
	}
}

// WithFailed is an option that sets the failed status for the transaction result.
func (f lightTransactionResultFactory) WithFailed(failed bool) LightTransactionResultOption {
	return func(g *LightTransactionResultGenerator, result *flow.LightTransactionResult) {
		result.Failed = failed
	}
}

// WithComputationUsed is an option that sets the computation used for the transaction result.
func (f lightTransactionResultFactory) WithComputationUsed(computationUsed uint64) LightTransactionResultOption {
	return func(g *LightTransactionResultGenerator, result *flow.LightTransactionResult) {
		result.ComputationUsed = computationUsed
	}
}

// LightTransactionResultGenerator generates light transaction results with consistent randomness.
type LightTransactionResultGenerator struct {
	*TransactionResultGenerator
}

func NewLightTransactionResultGenerator(
	txResults *TransactionResultGenerator,
) *LightTransactionResultGenerator {
	return &LightTransactionResultGenerator{
		TransactionResultGenerator: txResults,
	}
}

// Fixture generates a [flow.LightTransactionResult] with random data based on the provided options.
func (g *LightTransactionResultGenerator) Fixture(opts ...LightTransactionResultOption) flow.LightTransactionResult {
	result := flow.LightTransactionResult{
		TransactionID:   g.identifiers.Fixture(),
		ComputationUsed: g.random.Uint64InRange(1, 9999),
		Failed:          false,
	}

	for _, opt := range opts {
		opt(g, &result)
	}

	return result
}

// List generates a list of [flow.LightTransactionResult].
func (g *LightTransactionResultGenerator) List(n int, opts ...LightTransactionResultOption) []flow.LightTransactionResult {
	list := make([]flow.LightTransactionResult, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// ForTransactions generates a list of [flow.LightTransactionResult] for multiple transactions.
func (g *LightTransactionResultGenerator) ForTransactions(transactions []*flow.TransactionBody, opts ...LightTransactionResultOption) []flow.LightTransactionResult {
	list := make([]flow.LightTransactionResult, len(transactions))
	for i, tx := range transactions {
		nOpts := append(opts, LightTransactionResult.WithTransactionID(tx.ID()))
		list[i] = g.Fixture(nOpts...)
	}
	return list
}
