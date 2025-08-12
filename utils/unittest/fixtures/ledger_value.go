package fixtures

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
)

// LedgerValueGenerator generates ledger values with consistent randomness.
type LedgerValueGenerator struct {
	randomGen *RandomGenerator
}

// valueConfig holds the configuration for value generation.
type valueConfig struct {
	minSize int
	maxSize int
}

// WithSize returns an option to set the value size range [minSize, maxSize).
func (g *LedgerValueGenerator) WithSize(minSize, maxSize int) func(*valueConfig) {
	return func(config *valueConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// Fixture generates a single random ledger value.
func (g *LedgerValueGenerator) Fixture(t testing.TB, opts ...func(*valueConfig)) ledger.Value {
	config := &valueConfig{
		minSize: 1,
		maxSize: 8,
	}

	for _, opt := range opts {
		opt(config)
	}

	return g.generateValue(t, config.minSize, config.maxSize)
}

// List generates a list of random ledger values.
func (g *LedgerValueGenerator) List(t testing.TB, n int, opts ...func(*valueConfig)) []ledger.Value {
	values := make([]ledger.Value, n)
	for i := range n {
		values[i] = g.Fixture(t, opts...)
	}
	return values
}

// generateValue returns a random value with variable size (minByteSize <= size < maxByteSize).
func (g *LedgerValueGenerator) generateValue(t testing.TB, minByteSize, maxByteSize int) ledger.Value {
	require.LessOrEqual(t, minByteSize, maxByteSize, "minByteSize must be less than or equal to maxByteSize")

	var byteSize = maxByteSize
	if minByteSize < maxByteSize {
		byteSize = minByteSize + g.randomGen.Intn(maxByteSize-minByteSize)
	}
	return g.randomGen.RandomBytes(t, byteSize)
}
