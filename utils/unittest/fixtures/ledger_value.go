package fixtures

import (
	"github.com/onflow/flow-go/ledger"
)

// LedgerValue is the default options factory for [ledger.Value] generation.
var LedgerValue ledgerValueFactory

type ledgerValueFactory struct{}

type LedgerValueOption func(*LedgerValueGenerator, *valueConfig)

// valueConfig holds the configuration for value generation.
type valueConfig struct {
	minSize int
	maxSize int
}

// WithSize is an option that sets the value size range [minSize, maxSize).
func (f ledgerValueFactory) WithSize(minSize, maxSize int) LedgerValueOption {
	return func(g *LedgerValueGenerator, config *valueConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// LedgerValueGenerator generates ledger values with consistent randomness.
type LedgerValueGenerator struct {
	ledgerValueFactory

	random *RandomGenerator
}

func NewLedgerValueGenerator(
	random *RandomGenerator,
) *LedgerValueGenerator {
	return &LedgerValueGenerator{
		random: random,
	}
}

// Fixture generates a single random [ledger.Value].
func (g *LedgerValueGenerator) Fixture(opts ...LedgerValueOption) ledger.Value {
	config := &valueConfig{
		minSize: 1,
		maxSize: 8,
	}

	for _, opt := range opts {
		opt(g, config)
	}

	Assert(config.minSize <= config.maxSize, "minSize must be less than or equal to maxSize")

	return g.generateValue(config.minSize, config.maxSize)
}

// List generates a list of random [ledger.Value].
func (g *LedgerValueGenerator) List(n int, opts ...LedgerValueOption) []ledger.Value {
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
		byteSize = g.random.IntInRange(minByteSize, maxByteSize)
	}
	return g.random.RandomBytes(byteSize)
}
