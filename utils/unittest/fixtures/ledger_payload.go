package fixtures

import (
	"github.com/onflow/flow-go/ledger"
)

// LedgerPayloadGenerator generates ledger payloads with consistent randomness.
type LedgerPayloadGenerator struct {
	randomGen      *RandomGenerator
	ledgerValueGen *LedgerValueGenerator
}

func NewLedgerPayloadGenerator(
	randomGen *RandomGenerator,
	ledgerValueGen *LedgerValueGenerator,
) *LedgerPayloadGenerator {
	return &LedgerPayloadGenerator{
		randomGen:      randomGen,
		ledgerValueGen: ledgerValueGen,
	}
}

// payloadConfig holds the configuration for payload generation.
type payloadConfig struct {
	minSize int
	maxSize int
	value   ledger.Value
}

// WithSize is an option that sets the payload size range.
func (g *LedgerPayloadGenerator) WithSize(minSize, maxSize int) func(*payloadConfig) {
	return func(config *payloadConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// WithValue is an option that sets the value for the payload.
func (g *LedgerPayloadGenerator) WithValue(value ledger.Value) func(*payloadConfig) {
	return func(config *payloadConfig) {
		config.value = value
	}
}

// Fixture generates a single random [ledger.Payload].
func (g *LedgerPayloadGenerator) Fixture(opts ...func(*payloadConfig)) *ledger.Payload {
	config := &payloadConfig{
		minSize: 1,
		maxSize: 8,
	}

	for _, opt := range opts {
		opt(config)
	}

	Assert(config.minSize <= config.maxSize, "minSize must be less than or equal to maxSize")

	if config.value == nil {
		config.value = g.ledgerValueGen.Fixture(g.ledgerValueGen.WithSize(config.minSize, config.maxSize))
	}

	return g.generatePayload(config.minSize, config.maxSize, config.value)
}

// List generates a list of random [ledger.Payload].
func (g *LedgerPayloadGenerator) List(n int, opts ...func(*payloadConfig)) []*ledger.Payload {
	res := make([]*ledger.Payload, n)
	for i := range n {
		res[i] = g.Fixture(opts...)
	}
	return res
}

// generatePayload returns a random [ledger.Payload].
func (g *LedgerPayloadGenerator) generatePayload(minByteSize int, maxByteSize int, value ledger.Value) *ledger.Payload {
	keyByteSize := g.randomGen.IntInRange(minByteSize, maxByteSize)
	keydata := g.randomGen.RandomBytes(keyByteSize)
	key := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: keydata}}}

	return ledger.NewPayload(key, value)
}
