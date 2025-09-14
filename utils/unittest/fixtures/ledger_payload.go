package fixtures

import (
	"github.com/onflow/flow-go/ledger"
)

// LedgerPayload is the default options factory for [ledger.Payload] generation.
var LedgerPayload ledgerPayloadFactory

type ledgerPayloadFactory struct{}

type LedgerPayloadOption func(*LedgerPayloadGenerator, *payloadConfig)

// payloadConfig holds the configuration for payload generation.
type payloadConfig struct {
	minSize int
	maxSize int
	value   ledger.Value
}

// WithSize is an option that sets the payload size range.
func (f ledgerPayloadFactory) WithSize(minSize, maxSize int) LedgerPayloadOption {
	return func(g *LedgerPayloadGenerator, config *payloadConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// WithValue is an option that sets the value for the payload.
func (f ledgerPayloadFactory) WithValue(value ledger.Value) LedgerPayloadOption {
	return func(g *LedgerPayloadGenerator, config *payloadConfig) {
		config.value = value
	}
}

// LedgerPayloadGenerator generates ledger payloads with consistent randomness.
type LedgerPayloadGenerator struct {
	ledgerPayloadFactory

	random       *RandomGenerator
	ledgerValues *LedgerValueGenerator
}

func NewLedgerPayloadGenerator(
	random *RandomGenerator,
	ledgerValues *LedgerValueGenerator,
) *LedgerPayloadGenerator {
	return &LedgerPayloadGenerator{
		random:       random,
		ledgerValues: ledgerValues,
	}
}

// Fixture generates a single random [ledger.Payload].
func (g *LedgerPayloadGenerator) Fixture(opts ...LedgerPayloadOption) *ledger.Payload {
	config := &payloadConfig{
		minSize: 1,
		maxSize: 8,
	}

	for _, opt := range opts {
		opt(g, config)
	}

	Assert(config.minSize <= config.maxSize, "minSize must be less than or equal to maxSize")

	if config.value == nil {
		config.value = g.ledgerValues.Fixture(LedgerValue.WithSize(config.minSize, config.maxSize))
	}

	return g.generatePayload(config.minSize, config.maxSize, config.value)
}

// List generates a list of random [ledger.Payload].
func (g *LedgerPayloadGenerator) List(n int, opts ...LedgerPayloadOption) []*ledger.Payload {
	res := make([]*ledger.Payload, n)
	for i := range n {
		res[i] = g.Fixture(opts...)
	}
	return res
}

// generatePayload returns a random [ledger.Payload].
func (g *LedgerPayloadGenerator) generatePayload(minByteSize int, maxByteSize int, value ledger.Value) *ledger.Payload {
	keyByteSize := g.random.IntInRange(minByteSize, maxByteSize)
	keydata := g.random.RandomBytes(keyByteSize)
	key := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: keydata}}}

	return ledger.NewPayload(key, value)
}
