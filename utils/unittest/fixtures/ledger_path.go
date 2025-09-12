package fixtures

import (
	"github.com/onflow/flow-go/ledger"
)

// LedgerPathGenerator generates ledger paths with consistent randomness.
type LedgerPathGenerator struct {
	randomGen *RandomGenerator
}

func NewLedgerPathGenerator(
	randomGen *RandomGenerator,
) *LedgerPathGenerator {
	return &LedgerPathGenerator{
		randomGen: randomGen,
	}
}

// ledgerPathConfig holds the configuration for ledger path generation.
type ledgerPathConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// Fixture generates a single random [ledger.Path].
func (g *LedgerPathGenerator) Fixture(opts ...func(*ledgerPathConfig)) ledger.Path {
	config := &ledgerPathConfig{}

	for _, opt := range opts {
		opt(config)
	}

	var path ledger.Path
	pathData := g.randomGen.RandomBytes(ledger.PathLen)
	copy(path[:], pathData)
	return path
}

// List generates a list of random [ledger.Path].
func (g *LedgerPathGenerator) List(n int, opts ...func(*ledgerPathConfig)) []ledger.Path {
	paths := make([]ledger.Path, 0, n)
	alreadySelectPaths := make(map[ledger.Path]bool)
	i := 0
	for i < n {
		path := g.Fixture(opts...)

		// deduplicate
		if _, found := alreadySelectPaths[path]; !found {
			paths = append(paths, path)
			alreadySelectPaths[path] = true
			i++
		}
	}
	return paths
}
