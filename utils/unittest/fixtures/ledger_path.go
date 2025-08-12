package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
)

// LedgerPathGenerator generates ledger paths with consistent randomness.
type LedgerPathGenerator struct {
	randomGen *RandomGenerator
}

// Fixture generates a single random ledger path.
func (g *LedgerPathGenerator) Fixture(t testing.TB) ledger.Path {
	var path ledger.Path
	pathData := g.randomGen.RandomBytes(t, ledger.PathLen)
	copy(path[:], pathData)
	return path
}

// List generates a list of random ledger paths.
func (g *LedgerPathGenerator) List(t testing.TB, n int) []ledger.Path {
	paths := make([]ledger.Path, 0, n)
	alreadySelectPaths := make(map[ledger.Path]bool)
	i := 0
	for i < n {
		path := g.Fixture(t)

		// deduplicate
		if _, found := alreadySelectPaths[path]; !found {
			paths = append(paths, path)
			alreadySelectPaths[path] = true
			i++
		}
	}
	return paths
}
