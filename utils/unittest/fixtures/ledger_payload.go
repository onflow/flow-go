package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
)

// LedgerPayloadGenerator generates ledger payloads with consistent randomness.
type LedgerPayloadGenerator struct {
	randomGen      *RandomGenerator
	ledgerValueGen *LedgerValueGenerator
}

// payloadConfig holds the configuration for payload generation.
type payloadConfig struct {
	minSize int
	maxSize int
	value   ledger.Value
}

// WithSize returns an option to set the payload size range.
func (g *LedgerPayloadGenerator) WithSize(minSize, maxSize int) func(*payloadConfig) {
	return func(config *payloadConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// WithValue returns an option to set the value for the payload.
func (g *LedgerPayloadGenerator) WithValue(value ledger.Value) func(*payloadConfig) {
	return func(config *payloadConfig) {
		config.value = value
	}
}

// Fixture generates a single random ledger payload.
func (g *LedgerPayloadGenerator) Fixture(t testing.TB, opts ...func(*payloadConfig)) *ledger.Payload {
	config := &payloadConfig{
		minSize: 1,
		maxSize: 8,
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.value == nil {
		config.value = g.ledgerValueGen.Fixture(t, g.ledgerValueGen.WithSize(config.minSize, config.maxSize))
	}

	return g.generatePayload(t, config.minSize, config.maxSize, config.value)
}

// List generates a list of random ledger payloads.
func (g *LedgerPayloadGenerator) List(t testing.TB, n int, opts ...func(*payloadConfig)) []*ledger.Payload {
	res := make([]*ledger.Payload, n)
	for i := range n {
		res[i] = g.Fixture(t, opts...)
	}
	return res
}

// generatePayload returns a random payload.
func (g *LedgerPayloadGenerator) generatePayload(t testing.TB, minByteSize int, maxByteSize int, value ledger.Value) *ledger.Payload {
	keyByteSize := minByteSize + g.randomGen.Intn(maxByteSize-minByteSize)
	keydata := g.randomGen.RandomBytes(t, keyByteSize)
	key := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: keydata}}}

	return ledger.NewPayload(key, value)
}
