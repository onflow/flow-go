package gadgets

import (
	"fmt"
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
	orderedViews      []uint64
	lastFinalizedView uint64
}

// NewViews returns a new Views events gadget.
func NewViews() *Views {
	views := &Views{
		callbacks:    make(map[uint64][]events.OnViewCallback),
		orderedViews: make([]uint64, 0),
	}
	return views
}

// OnView registers the callback for the given view.
func (v *Views) OnView(view uint64, callback events.OnViewCallback) {
	v.Lock()
	defer v.Unlock()

	// index a view the first time we see it
	callbacks := v.callbacks[view]
	if len(callbacks) == 0 {
		v.indexView(view)
	}
	v.callbacks[view] = append(callbacks, callback)
}

func (v *Views) indexView(view uint64) {
	// no indexed views
	if len(v.orderedViews) == 0 {
		v.orderedViews = append(v.orderedViews, view)
		return
	}

	// find the insertion index in the ordered list of views
	// start with higher views to match typical usage patterns
	insertAt := 0
	for i := len(v.orderedViews) - 1; i >= 0; i-- {
		viewI := v.orderedViews[i]
		if view > viewI {
			insertAt = i + 1
			break
		}
	}
	// shift the list right, insert the new view
	v.orderedViews = append(v.orderedViews, 0)                   // add capacity (will be overwritten)
	copy(v.orderedViews[insertAt+1:], v.orderedViews[insertAt:]) // shift larger views right
	v.orderedViews[insertAt] = view                              // insert new view
}

// BlockFinalized handles block finalized protocol events, triggering view
// callbacks as needed.
func (v *Views) BlockFinalized(block *flow.Header) {
	v.Lock()
	defer v.Unlock()

	blockView := block.View

	fmt.Println("BlockFinalized")
	fmt.Println(v.orderedViews)
	fmt.Println(v.callbacks)

	// the index (inclusive) of the lowest view which should be kept
	cutoff := 0
	for i, view := range v.orderedViews {
		if view > blockView {
			break
		}
		for _, callback := range v.callbacks[view] {
			callback(block)
		}
		delete(v.callbacks, view)
		cutoff = i + 1
	}

	// we have no other queued view callbacks
	if cutoff >= len(v.orderedViews) {
		if len(v.orderedViews) > 0 {
			v.orderedViews = []uint64{}
			return
		}
	}
	// remove view callbacks which have been invoked
	v.orderedViews = v.orderedViews[cutoff:]
}
