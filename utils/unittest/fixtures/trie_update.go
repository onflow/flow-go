package fixtures

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// TrieUpdateGenerator generates trie updates with consistent randomness.
type TrieUpdateGenerator struct {
	randomGen *RandomGenerator
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

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Generate paths and payloads if not provided
	if config.paths == nil {
		config.paths = g.RandomPaths(t, config.numPaths)
	}
	if config.payloads == nil {
		config.payloads = g.RandomPayloads(t, config.numPaths, config.minSize, config.maxSize)
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

// RandomPaths generates n random (no repetition)
func (g *TrieUpdateGenerator) RandomPaths(t testing.TB, n int) []ledger.Path {
	paths := make([]ledger.Path, 0, n)
	alreadySelectPaths := make(map[ledger.Path]bool)
	i := 0
	for i < n {
		var path ledger.Path
		pathData := g.randomGen.RandomBytes(t, ledger.PathLen)
		copy(path[:], pathData)

		// deduplicate
		if _, found := alreadySelectPaths[path]; !found {
			paths = append(paths, path)
			alreadySelectPaths[path] = true
			i++
		}
	}
	return paths
}

// RandomPayload returns a random payload
func (g *TrieUpdateGenerator) RandomPayload(t testing.TB, minByteSize int, maxByteSize int) *ledger.Payload {
	keyByteSize := minByteSize + g.randomGen.Intn(maxByteSize-minByteSize)
	keydata := g.randomGen.RandomBytes(t, keyByteSize)
	key := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: keydata}}}

	valueByteSize := minByteSize + g.randomGen.Intn(maxByteSize-minByteSize)
	valuedata := g.randomGen.RandomBytes(t, valueByteSize)
	value := ledger.Value(valuedata)
	return ledger.NewPayload(key, value)
}

// RandomPayloads returns n random payloads
func (g *TrieUpdateGenerator) RandomPayloads(t testing.TB, n int, minByteSize int, maxByteSize int) []*ledger.Payload {
	res := make([]*ledger.Payload, 0, n)
	for range n {
		res = append(res, g.RandomPayload(t, minByteSize, maxByteSize))
	}
	return res
}

// RandomValues returns n random values with variable sizes (minByteSize <= size < maxByteSize)
func (g *TrieUpdateGenerator) RandomValues(t testing.TB, n int, minByteSize, maxByteSize int) []ledger.Value {
	require.LessOrEqual(t, minByteSize, maxByteSize, "minByteSize must be less than or equal to maxByteSize")

	values := make([]ledger.Value, 0, n)
	for range n {
		var byteSize = maxByteSize
		if minByteSize < maxByteSize {
			byteSize = minByteSize + g.randomGen.Intn(maxByteSize-minByteSize)
		}
		values = append(values, g.randomGen.RandomBytes(t, byteSize))
	}
	return values
}
