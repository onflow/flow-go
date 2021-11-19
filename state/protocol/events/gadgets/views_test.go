package gadgets

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestViews(t *testing.T) {
	views := NewViews()

	calls := []int{}
	order := 0
	makeCallback := func() events.OnViewCallback {
		corder := order
		order++
		return func(*flow.Header) {
			calls = append(calls, corder)
		}
	}

	for i := 1; i <= 100; i++ {
		views.OnView(uint64(i), makeCallback())
		views.OnView(uint64(i), makeCallback())
	}

	views.OnView(101, makeCallback())

	block := unittest.BlockHeaderFixture()
	block.View = 100
	views.BlockFinalized(&block)

	// ensure callbacks were invoked correctly
	assert.Equal(t, 200, len(calls))

	assert.True(t, sort.IntsAreSorted(calls), "callbacks executed in wrong order")

	// ensure map is cleared appropriately (only view 101 should remain)
	assert.Equal(t, 1, len(views.callbacks))
}

// BenchmarkViews_BlockFinalized benchmarks the performance of BlockFinalized.
func BenchmarkViews_BlockFinalized(b *testing.B) {
	views := NewViews()

	// DKG on mainnet has 6000 views, and 600 callbacks, this tests the upper
	// bound of that magnitude
	for i := 0; i < 1000; i++ {
		views.OnView(rand.Uint64(), func(_ *flow.Header) {})
	}

	block := unittest.BlockHeaderFixture()
	block.View = 1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		views.BlockFinalized(&block)
	}
}
