package fixtures

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// TrieUpdate is the default options factory for [ledger.TrieUpdate] generation.
var TrieUpdate trieUpdateFactory

type trieUpdateFactory struct{}

type TrieUpdateOption func(*TrieUpdateGenerator, *trieUpdateConfig)

// trieUpdateConfig holds the configuration for trie update generation.
type trieUpdateConfig struct {
	trieUpdate *ledger.TrieUpdate
	numPaths   int
	minSize    int
	maxSize    int
}

// WithRootHash is an option that sets the root hash for the trie update.
func (f trieUpdateFactory) WithRootHash(rootHash ledger.RootHash) TrieUpdateOption {
	return func(g *TrieUpdateGenerator, config *trieUpdateConfig) {
		config.trieUpdate.RootHash = rootHash
	}
}

// WithPaths is an option that sets the paths for the trie update.
func (f trieUpdateFactory) WithPaths(paths ...ledger.Path) TrieUpdateOption {
	return func(g *TrieUpdateGenerator, config *trieUpdateConfig) {
		config.trieUpdate.Paths = paths
	}
}

// WithPayloads is an option that sets the payloads for the trie update.
func (f trieUpdateFactory) WithPayloads(payloads ...*ledger.Payload) TrieUpdateOption {
	return func(g *TrieUpdateGenerator, config *trieUpdateConfig) {
		config.trieUpdate.Payloads = payloads
	}
}

// WithNumPaths is an option that sets the number of paths for the trie update.
func (f trieUpdateFactory) WithNumPaths(numPaths int) TrieUpdateOption {
	return func(g *TrieUpdateGenerator, config *trieUpdateConfig) {
		config.numPaths = numPaths
	}
}

// WithPayloadSize is an option that sets the payload size range for the trie update.
func (f trieUpdateFactory) WithPayloadSize(minSize, maxSize int) TrieUpdateOption {
	return func(g *TrieUpdateGenerator, config *trieUpdateConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// TrieUpdateGenerator generates trie updates with consistent randomness.
type TrieUpdateGenerator struct {
	trieUpdateFactory

	random         *RandomGenerator
	ledgerPaths    *LedgerPathGenerator
	ledgerPayloads *LedgerPayloadGenerator
}

func NewTrieUpdateGenerator(
	random *RandomGenerator,
	ledgerPaths *LedgerPathGenerator,
	ledgerPayloads *LedgerPayloadGenerator,
) *TrieUpdateGenerator {
	return &TrieUpdateGenerator{
		random:         random,
		ledgerPaths:    ledgerPaths,
		ledgerPayloads: ledgerPayloads,
	}
}

// Fixture generates a [ledger.TrieUpdate] with random data based on the provided options.
func (g *TrieUpdateGenerator) Fixture(opts ...TrieUpdateOption) *ledger.TrieUpdate {
	config := &trieUpdateConfig{
		trieUpdate: &ledger.TrieUpdate{
			RootHash: testutils.RootHashFixture(),
			Paths:    nil,
			Payloads: nil,
		},
		numPaths: 2,
		minSize:  1,
		maxSize:  8,
	}

	for _, opt := range opts {
		opt(g, config)
	}

	// Generate paths and payloads if not provided
	if config.trieUpdate.Paths == nil {
		config.trieUpdate.Paths = g.ledgerPaths.List(config.numPaths)
	}
	if config.trieUpdate.Payloads == nil {
		config.trieUpdate.Payloads = g.ledgerPayloads.List(config.numPaths, LedgerPayload.WithSize(config.minSize, config.maxSize))
	}

	Assertf(len(config.trieUpdate.Paths) == len(config.trieUpdate.Payloads), "paths and payloads must have the same length")

	return config.trieUpdate
}

// List generates a list of [ledger.TrieUpdate].
func (g *TrieUpdateGenerator) List(n int, opts ...TrieUpdateOption) []*ledger.TrieUpdate {
	list := make([]*ledger.TrieUpdate, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
