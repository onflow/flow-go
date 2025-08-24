package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// TrieUpdateGenerator generates trie updates with consistent randomness.
type TrieUpdateGenerator struct {
	randomGen        *RandomGenerator
	ledgerPathGen    *LedgerPathGenerator
	ledgerPayloadGen *LedgerPayloadGenerator
}

// trieUpdateConfig holds the configuration for trie update generation.
type trieUpdateConfig struct {
	rootHash ledger.RootHash
	paths    []ledger.Path
	payloads []*ledger.Payload
	numPaths int
	minSize  int
	maxSize  int
}

// WithRootHash returns an option to set the root hash for the trie update.
func (g *TrieUpdateGenerator) WithRootHash(rootHash ledger.RootHash) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.rootHash = rootHash
	}
}

// WithPaths returns an option to set the paths for the trie update.
func (g *TrieUpdateGenerator) WithPaths(paths []ledger.Path) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.paths = paths
	}
}

// WithPayloads returns an option to set the payloads for the trie update.
func (g *TrieUpdateGenerator) WithPayloads(payloads []*ledger.Payload) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.payloads = payloads
	}
}

// WithNumPaths returns an option to set the number of paths for the trie update.
func (g *TrieUpdateGenerator) WithNumPaths(numPaths int) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.numPaths = numPaths
	}
}

// WithPayloadSize returns an option to set the payload size range for the trie update.
func (g *TrieUpdateGenerator) WithPayloadSize(minSize, maxSize int) func(*trieUpdateConfig) {
	return func(config *trieUpdateConfig) {
		config.minSize = minSize
		config.maxSize = maxSize
	}
}

// Fixture generates a trie update with optional configuration.
func (g *TrieUpdateGenerator) Fixture(t testing.TB, opts ...func(*trieUpdateConfig)) *ledger.TrieUpdate {
	config := &trieUpdateConfig{
		rootHash: testutils.RootHashFixture(),
		paths:    nil,
		payloads: nil,
		numPaths: 2,
		minSize:  1,
		maxSize:  8,
	}

	for _, opt := range opts {
		opt(config)
	}

	// Generate paths and payloads if not provided
	if config.paths == nil {
		config.paths = g.ledgerPathGen.List(t, config.numPaths)
	}
	if config.payloads == nil {
		config.payloads = g.ledgerPayloadGen.List(t, config.numPaths, g.ledgerPayloadGen.WithSize(config.minSize, config.maxSize))
	}

	return &ledger.TrieUpdate{
		RootHash: config.rootHash,
		Paths:    config.paths,
		Payloads: config.payloads,
	}
}

// List generates a list of trie updates.
func (g *TrieUpdateGenerator) List(t testing.TB, n int, opts ...func(*trieUpdateConfig)) []*ledger.TrieUpdate {
	list := make([]*ledger.TrieUpdate, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
