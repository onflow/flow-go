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
	mu      sync.Mutex
	heights map[uint64][]func()
}

// NewHeights returns a new Heights events gadget.
func NewHeights() *Heights {
	heights := &Heights{
		heights: make(map[uint64][]func()),
	}
	return heights
}

// BlockFinalized handles block finalized protocol events, triggering height
// callbacks as needed.
func (g *Heights) BlockFinalized(block *flow.Header) {
	g.mu.Lock()
	defer g.mu.Unlock()

	final := block.Height
	callbacks, ok := g.heights[final]
	if !ok {
		return
	}

	for _, callback := range callbacks {
		callback()
	}

	// remove all invalid heights
	for height := range g.heights {
		if height <= final {
			delete(g.heights, height)
		}
	}
}

// OnHeight registers the callback for the given height.
func (g *Heights) OnHeight(height uint64, callback func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.heights[height] = append(g.heights[height], callback)
}
