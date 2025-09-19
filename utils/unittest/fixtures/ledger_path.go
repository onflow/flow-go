package fixtures

import (
	"github.com/onflow/flow-go/ledger"
)

// LedgerPath is the default options factory for [ledger.Path] generation.
var LedgerPath ledgerPathFactory

type ledgerPathFactory struct{}

type LedgerPathOption func(*LedgerPathGenerator, *ledgerPathConfig)

// ledgerPathConfig holds the configuration for ledger path generation.
type ledgerPathConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// LedgerPathGenerator generates ledger paths with consistent randomness.
type LedgerPathGenerator struct {
	ledgerPathFactory //nolint:unused

	random *RandomGenerator
}

func NewLedgerPathGenerator(
	random *RandomGenerator,
) *LedgerPathGenerator {
	return &LedgerPathGenerator{
		random: random,
	}
}

// Fixture generates a single random [ledger.Path].
func (g *LedgerPathGenerator) Fixture(opts ...LedgerPathOption) ledger.Path {
	config := &ledgerPathConfig{}

	for _, opt := range opts {
		opt(g, config)
	}

	var path ledger.Path
	pathData := g.random.RandomBytes(ledger.PathLen)
	copy(path[:], pathData)
	return path
}

// List generates a list of random [ledger.Path].
func (g *LedgerPathGenerator) List(n int, opts ...LedgerPathOption) []ledger.Path {
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
