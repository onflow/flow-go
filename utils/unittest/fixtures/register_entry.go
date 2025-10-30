package fixtures

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// RegisterEntry is the default options factory for [flow.RegisterEntry] generation.
var RegisterEntry registerEntryFactory

type registerEntryFactory struct{}

type RegisterEntryOption func(*RegisterEntryGenerator, *flow.RegisterEntry)

func (f registerEntryFactory) WithKey(key flow.RegisterID) RegisterEntryOption {
	return func(g *RegisterEntryGenerator, entry *flow.RegisterEntry) {
		entry.Key = key
	}
}

func (f registerEntryFactory) WithValue(value flow.RegisterValue) RegisterEntryOption {
	return func(g *RegisterEntryGenerator, entry *flow.RegisterEntry) {
		entry.Value = value
	}
}

func (f registerEntryFactory) WithPayload(payload *ledger.Payload) RegisterEntryOption {
	return func(g *RegisterEntryGenerator, entry *flow.RegisterEntry) {
		key, value, err := convert.PayloadToRegister(payload)
		NoError(err)

		entry.Key = key
		entry.Value = value
	}
}

type RegisterEntryGenerator struct {
	registerEntryFactory

	random         *RandomGenerator
	ledgerPayloads *LedgerPayloadGenerator
}

func NewRegisterEntryGenerator(
	random *RandomGenerator,
	ledgerPayloads *LedgerPayloadGenerator,
) *RegisterEntryGenerator {
	return &RegisterEntryGenerator{
		random:         random,
		ledgerPayloads: ledgerPayloads,
	}
}

// Fixture generates a [flow.RegisterEntry] with random data based on the provided options.
func (g *RegisterEntryGenerator) Fixture(opts ...RegisterEntryOption) flow.RegisterEntry {
	payload := g.ledgerPayloads.Fixture()
	key, value, err := convert.PayloadToRegister(payload)
	NoError(err)

	entry := flow.RegisterEntry{
		Key:   key,
		Value: value,
	}

	for _, opt := range opts {
		opt(g, &entry)
	}

	return entry
}

// List generates a list of [flow.RegisterEntry].
func (g *RegisterEntryGenerator) List(n int, opts ...RegisterEntryOption) []flow.RegisterEntry {
	entries := make([]flow.RegisterEntry, n)
	for i := range n {
		entries[i] = g.Fixture(opts...)
	}
	return entries
}
