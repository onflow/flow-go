package gadgets

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"gotest.tools/assert"
)

func TestHeights(t *testing.T) {
	heights := NewHeights()

	calls := 0

	bad := func() { t.Fail() } // should not be called
	good := func() { calls++ } // should be called

	// we will start finalizing at 2, and finalize through 4. Registered
	// heights 2-4 should be invoked, 1 and 5 should not be

	heights.OnHeight(1, bad) // we start finalizing after block 1
	heights.OnHeight(2, good)
	heights.OnHeight(2, good) // register 2 callbacks for height 2
	heights.OnHeight(4, good)
	heights.OnHeight(5, bad) // we won't finalize block 5

	for height := uint64(2); height <= 4; height++ {
		block := unittest.BlockHeaderFixture()
		block.Height = height
		heights.BlockFinalized(&block)
	}

	// ensure callbacks were invoked correctly
	assert.Equal(t, 3, calls)

	// ensure map is cleared appropriately (only height 5 should remain)
	assert.Equal(t, 1, len(heights.heights))
}
