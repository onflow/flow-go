package gadgets

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/events"
)

// Heights is a protocol events consumer that provides an interface to
// subscribe to callbacks when chain state reaches a particular height.
type Heights struct {
	events.Noop
	mu              sync.Mutex
	heights         map[uint64][]func()
	finalizedHeight uint64
}

// NewHeights returns a new Heights events gadget.
func NewHeights() *Heights {
	heights := &Heights{
		heights:         make(map[uint64][]func()),
		finalizedHeight: 0,
	}
	return heights
}

// BlockFinalized handles block finalized protocol events, triggering height
// callbacks as needed.
func (g *Heights) BlockFinalized(block *flow.Header) {
	g.mu.Lock()
	defer g.mu.Unlock()

	lastFinalizedHeight := g.finalizedHeight
	final := block.Height
	g.finalizedHeight = final

	callbacks := g.heights[final]
	for _, callback := range callbacks { // safe when callbacks is nil
		callback()
	}

	// to safely and efficiently prune the height callbacks, only prune
	// potentially stale (below latest finalized) heights the first time
	// we observe a finalized block

	// typical case, we are finalized the child of the last observed - we
	// only need to clear the callbacks for the current height
	if final == lastFinalizedHeight+1 {
		delete(g.heights, final)
		return
	}

	// non-typical case - there is a gap between our "last finalized height"
	// and this block - this means this is the first block we are observing.
	// this is the only time it possible to have stale heights, therefore prune
	// the whole heights map
	for height := range g.heights {
		if height <= final {
			delete(g.heights, height)
		}
	}
}

// OnHeight registers the callback for the given height, only if the height has
// not already been finalized.
func (g *Heights) OnHeight(height uint64, callback func()) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// skip already finalized heights - they will never be invoked
	if height <= g.finalizedHeight {
		return
	}
	g.heights[height] = append(g.heights[height], callback)
}
