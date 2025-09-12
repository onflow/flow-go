package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionResultGenerator generates transaction results with consistent randomness.
type TransactionResultGenerator struct {
	randomGen     *RandomGenerator
	identifierGen *IdentifierGenerator
}

func NewTransactionResultGenerator(
	randomGen *RandomGenerator,
	identifierGen *IdentifierGenerator,
) *TransactionResultGenerator {
	return &TransactionResultGenerator{
		randomGen:     randomGen,
		identifierGen: identifierGen,
	}
}

// WithTransactionID is an option that sets the transaction ID for the transaction result.
func (g *TransactionResultGenerator) WithTransactionID(transactionID flow.Identifier) func(*flow.TransactionResult) {
	return func(result *flow.TransactionResult) {
		result.TransactionID = transactionID
	}
}

// WithErrorMessage is an option that sets the error message for the transaction result.
func (g *TransactionResultGenerator) WithErrorMessage(errorMessage string) func(*flow.TransactionResult) {
	return func(result *flow.TransactionResult) {
		result.ErrorMessage = errorMessage
	}
}

// WithComputationUsed is an option that sets the computation used for the transaction result.
func (g *TransactionResultGenerator) WithComputationUsed(computationUsed uint64) func(*flow.TransactionResult) {
	return func(result *flow.TransactionResult) {
		result.ComputationUsed = computationUsed
	}
}

// Fixture generates a [flow.TransactionResult] with random data based on the provided options.
func (g *TransactionResultGenerator) Fixture(opts ...func(*flow.TransactionResult)) flow.TransactionResult {
	result := flow.TransactionResult{
		TransactionID:   g.identifierGen.Fixture(),
		ComputationUsed: g.randomGen.Uint64InRange(1, 9999),
		ErrorMessage:    "",
	}

	for _, opt := range opts {
		opt(&result)
	}

	return result
}

// List generates a list of [flow.TransactionResult].
func (g *TransactionResultGenerator) List(n int, opts ...func(*flow.TransactionResult)) []flow.TransactionResult {
	list := make([]flow.TransactionResult, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// LightTransactionResultGenerator generates light transaction results with consistent randomness.
type LightTransactionResultGenerator struct {
	*TransactionResultGenerator
}

func NewLightTransactionResultGenerator(
	transactionResultGen *TransactionResultGenerator,
) *LightTransactionResultGenerator {
	return &LightTransactionResultGenerator{
		TransactionResultGenerator: transactionResultGen,
	}
}

// WithTransactionID is an option that sets the transaction ID for the transaction result.
func (g *LightTransactionResultGenerator) WithTransactionID(transactionID flow.Identifier) func(*flow.LightTransactionResult) {
	return func(result *flow.LightTransactionResult) {
		result.TransactionID = transactionID
	}
}

// WithFailed is an option that sets the failed status for the transaction result.
func (g *LightTransactionResultGenerator) WithFailed(failed bool) func(*flow.LightTransactionResult) {
	return func(result *flow.LightTransactionResult) {
		result.Failed = failed
	}
}

// WithComputationUsed is an option that sets the computation used for the transaction result.
func (g *LightTransactionResultGenerator) WithComputationUsed(computationUsed uint64) func(*flow.LightTransactionResult) {
	return func(result *flow.LightTransactionResult) {
		result.ComputationUsed = computationUsed
	}
}

// Fixture generates a [flow.LightTransactionResult] with random data based on the provided options.
func (g *LightTransactionResultGenerator) Fixture(opts ...func(*flow.LightTransactionResult)) flow.LightTransactionResult {
	result := flow.LightTransactionResult{
		TransactionID:   g.identifierGen.Fixture(),
		ComputationUsed: g.randomGen.Uint64InRange(1, 9999),
		Failed:          false,
	}

	for _, opt := range opts {
		opt(&result)
	}

	return result
}

// List generates a list of [flow.LightTransactionResult].
func (g *LightTransactionResultGenerator) List(n int, opts ...func(*flow.LightTransactionResult)) []flow.LightTransactionResult {
	list := make([]flow.LightTransactionResult, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
