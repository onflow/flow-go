package fixtures

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// TrieUpdateGenerator generates trie updates with consistent randomness.
type TrieUpdateGenerator struct {
	randomGen        *RandomGenerator
	ledgerPathGen    *LedgerPathGenerator
	ledgerPayloadGen *LedgerPayloadGenerator
}

func NewTrieUpdateGenerator(
	randomGen *RandomGenerator,
	ledgerPathGen *LedgerPathGenerator,
	ledgerPayloadGen *LedgerPayloadGenerator,
) *TrieUpdateGenerator {
	return &TrieUpdateGenerator{
		randomGen:        randomGen,
		ledgerPathGen:    ledgerPathGen,
		ledgerPayloadGen: ledgerPayloadGen,
	}
}

// trieUpdateConfig holds the configuration for trie update generation.
type trieUpdateConfig struct {
	trieUpdate *ledger.TrieUpdate
	numPaths   int
	minSize    int
	maxSize    int
}

// WithRootHash is an option that sets the root hash for the trie update.
func (g *TrieUpdateGenerator) WithRootHash(rootHash ledger.RootHash) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.trieUpdate.RootHash = rootHash
	}
}

// WithPaths is an option that sets the paths for the trie update.
func (g *TrieUpdateGenerator) WithPaths(paths []ledger.Path) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.trieUpdate.Paths = paths
	}
}

// WithPayloads is an option that sets the payloads for the trie update.
func (g *TrieUpdateGenerator) WithPayloads(payloads []*ledger.Payload) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.trieUpdate.Payloads = payloads
	}
}

// WithNumPaths is an option that sets the number of paths for the trie update.
func (g *TrieUpdateGenerator) WithNumPaths(numPaths int) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.numPaths = numPaths
	}
}

// WithPayloadSize is an option that sets the payload size range for the trie update.
func (g *TrieUpdateGenerator) WithPayloadSize(minSize, maxSize int) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// Fixture generates a [ledger.TrieUpdate] with random data based on the provided options.
func (g *TrieUpdateGenerator) Fixture(opts ...func(*trieUpdateConfig)) *ledger.TrieUpdate {
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
		opt(config)
	}

	// Generate paths and payloads if not provided
	if config.trieUpdate.Paths == nil {
		config.trieUpdate.Paths = g.ledgerPathGen.List(config.numPaths)
	}
	if config.trieUpdate.Payloads == nil {
		config.trieUpdate.Payloads = g.ledgerPayloadGen.List(config.numPaths, g.ledgerPayloadGen.WithSize(config.minSize, config.maxSize))
	}

	return config.trieUpdate
}

// List generates a list of [ledger.TrieUpdate].
func (g *TrieUpdateGenerator) List(n int, opts ...func(*trieUpdateConfig)) []*ledger.TrieUpdate {
	list := make([]*ledger.TrieUpdate, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
