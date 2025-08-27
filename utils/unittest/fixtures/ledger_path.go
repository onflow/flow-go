package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
)

// LedgerPathGenerator generates ledger paths with consistent randomness.
type LedgerPathGenerator struct {
	randomGen *RandomGenerator
}

// ledgerPathConfig holds the configuration for ledger path generation.
type ledgerPathConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// Fixture generates a single random ledger path.
func (g *LedgerPathGenerator) Fixture(t testing.TB, opts ...func(*ledgerPathConfig)) ledger.Path {
	config := &ledgerPathConfig{}

	for _, opt := range opts {
		opt(config)
	}

	var path ledger.Path
	pathData := g.randomGen.RandomBytes(t, ledger.PathLen)
	copy(path[:], pathData)
	return path
}

// List generates a list of random ledger paths.
func (g *LedgerPathGenerator) List(t testing.TB, n int, opts ...func(*ledgerPathConfig)) []ledger.Path {
	paths := make([]ledger.Path, 0, n)
	alreadySelectPaths := make(map[ledger.Path]bool)
	i := 0
	for i < n {
		path := g.Fixture(t, opts...)

		// deduplicate
		if _, found := alreadySelectPaths[path]; !found {
			paths = append(paths, path)
			alreadySelectPaths[path] = true
			i++
		}
	}
	return paths
}
