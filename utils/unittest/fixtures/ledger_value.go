package fixtures

import (
	"github.com/onflow/flow-go/ledger"
)

// LedgerValueGenerator generates ledger values with consistent randomness.
type LedgerValueGenerator struct {
	randomGen *RandomGenerator
}

func NewLedgerValueGenerator(
	randomGen *RandomGenerator,
) *LedgerValueGenerator {
	return &LedgerValueGenerator{
		randomGen: randomGen,
	}
}

// valueConfig holds the configuration for value generation.
type valueConfig struct {
	minSize int
	maxSize int
}

// WithSize is an option that sets the value size range [minSize, maxSize).
func (g *LedgerValueGenerator) WithSize(minSize, maxSize int) func(*valueConfig) {
	return func(config *valueConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// Fixture generates a single random [ledger.Value].
func (g *LedgerValueGenerator) Fixture(opts ...func(*valueConfig)) ledger.Value {
	config := &valueConfig{
		minSize: 1,
		maxSize: 8,
	}

	for _, opt := range opts {
		opt(config)
	}

	Assert(config.minSize <= config.maxSize, "minSize must be less than or equal to maxSize")

	return g.generateValue(config.minSize, config.maxSize)
}

// List generates a list of random [ledger.Value].
func (g *LedgerValueGenerator) List(n int, opts ...func(*valueConfig)) []ledger.Value {
	values := make([]ledger.Value, n)
	for i := range n {
		values[i] = g.Fixture(opts...)
	}
	return values
}

// generateValue returns a random [ledger.Value] with variable size (minByteSize <= size < maxByteSize).
func (g *LedgerValueGenerator) generateValue(minByteSize, maxByteSize int) ledger.Value {
	byteSize := maxByteSize
	if minByteSize < maxByteSize {
		byteSize = g.randomGen.IntInRange(minByteSize, maxByteSize)
	}
	return g.randomGen.RandomBytes(byteSize)
}
