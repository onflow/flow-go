package gadgets

import (
	"sort"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/events"
)

// Views is a protocol events consumer that provides an interface to subscribe
// to callbacks when consensus reaches a particular view. When a callback is
// registered for a view, it will be invoked when the first block with
// block.View >= V is finalized. Callbacks for earlier views are executed before
// callbacks for later views, and callbacks for the same view are executed on a
// FIFO basis.
type Views struct {
	sync.Mutex
	events.Noop
	callbacks         map[uint64][]events.OnViewCallback
	lastFinalizedView uint64
}

// NewViews returns a new Views events gadget.
func NewViews() *Views {
	views := &Views{
		callbacks: make(map[uint64][]events.OnViewCallback),
	}
	return views
}

// OnView registers the callback for the given view.
func (v *Views) OnView(view uint64, callback events.OnViewCallback) {
	v.Lock()
	defer v.Unlock()

	// do not subscribe to a view that has already been finalized
	if view <= v.lastFinalizedView {
		return
	}

	v.callbacks[view] = append(v.callbacks[view], callback)
}

// BlockFinalized handles block finalized protocol events, triggering view
// callbacks as needed.
func (v *Views) BlockFinalized(block *flow.Header) {
	v.Lock()
	defer v.Unlock()

	v.lastFinalizedView = block.View

	finalizedViews := []int{}
	for view := range v.callbacks {
		if view <= block.View {
			finalizedViews = append(finalizedViews, int(view))
		}
	}

	sort.Ints(finalizedViews)

	for _, view := range finalizedViews {
		for _, callback := range v.callbacks[uint64(view)] {
			callback(block)
		}
		delete(v.callbacks, uint64(view))
	}
}
