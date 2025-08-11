package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionResultGenerator generates transaction results with consistent randomness.
type TransactionResultGenerator struct {
	randomGen     *RandomGenerator
	identifierGen *IdentifierGenerator
}

// transactionResultConfig holds the configuration for transaction result generation.
type transactionResultConfig struct {
	transactionID   flow.Identifier
	errorMessage    string
	computationUsed uint64
	failed          bool
}

// WithTransactionID returns an option to set the transaction ID for the transaction result.
func (g *TransactionResultGenerator) WithTransactionID(transactionID flow.Identifier) func(*transactionResultConfig) {
	return func(config *transactionResultConfig) {
		config.transactionID = transactionID
	}
}

// WithErrorMessage returns an option to set the error message for the transaction result.
func (g *TransactionResultGenerator) WithErrorMessage(errorMessage string) func(*transactionResultConfig) {
	return func(config *transactionResultConfig) {
		config.errorMessage = errorMessage
		config.failed = len(config.errorMessage) > 0
	}
}

// WithFailed returns an option to set the failed status for the transaction result.
func (g *LightTransactionResultGenerator) WithFailed(failed bool) func(*transactionResultConfig) {
	return func(config *transactionResultConfig) {
		config.failed = failed
		if failed {
			config.errorMessage = "failed"
		} else {
			config.errorMessage = ""
		}
	}
}

// WithComputationUsed returns an option to set the computation used for the transaction result.
func (g *TransactionResultGenerator) WithComputationUsed(computationUsed uint64) func(*transactionResultConfig) {
	return func(config *transactionResultConfig) {
		config.computationUsed = computationUsed
	}
}

// Fixture generates a transaction result with optional configuration.
func (g *TransactionResultGenerator) Fixture(t testing.TB, opts ...func(*transactionResultConfig)) flow.TransactionResult {
	config := &transactionResultConfig{
		transactionID:   g.identifierGen.Fixture(t),
		errorMessage:    "",
		computationUsed: g.randomGen.Uint64InRange(1, 10_000),
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	return flow.TransactionResult{
		TransactionID:   config.transactionID,
		ErrorMessage:    config.errorMessage,
		ComputationUsed: config.computationUsed,
	}
}

// List generates a list of transaction results.
func (g *TransactionResultGenerator) List(t testing.TB, n int, opts ...func(*transactionResultConfig)) []flow.TransactionResult {
	list := make([]flow.TransactionResult, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}

// LightTransactionResultGenerator generates light transaction results with consistent randomness.
type LightTransactionResultGenerator struct {
	*TransactionResultGenerator
}

// Fixture generates a light transaction result with optional configuration.
func (g *LightTransactionResultGenerator) Fixture(t testing.TB, opts ...func(*transactionResultConfig)) flow.LightTransactionResult {
	config := &transactionResultConfig{
		transactionID:   g.identifierGen.Fixture(t),
		computationUsed: g.randomGen.Uint64InRange(1, 10_000),
		failed:          false,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	return flow.LightTransactionResult{
		TransactionID:   config.transactionID,
		Failed:          config.failed,
		ComputationUsed: config.computationUsed,
	}
}

// List generates a list of transaction results.
func (g *LightTransactionResultGenerator) List(t testing.TB, n int, opts ...func(*transactionResultConfig)) []flow.LightTransactionResult {
	list := make([]flow.LightTransactionResult, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
