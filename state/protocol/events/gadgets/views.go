package gadgets

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/events"
)

// Views is a protocol events consumer that provides an interface to
// subscribe to callbacks when chain state reaches a particular view.
type Views struct {
	sync.Mutex
	events.Noop
	views       map[uint64][]events.OnViewCallback
	currentView uint64
}

// NewViews returns a new Views events gadget.
func NewViews() *Views {
	views := &Views{
		views: make(map[uint64][]events.OnViewCallback),
	}
	return views
}

// OnView registers the callback for the given view.
func (v *Views) OnView(view uint64, callback events.OnViewCallback) {
	v.Lock()
	defer v.Unlock()
	v.views[view] = append(v.views[view], callback)
}

// BlockFinalized handles block finalized protocol events, triggering view
// callbacks as needed.
func (v *Views) BlockFinalized(block *flow.Header) {
	v.Lock()
	defer v.Unlock()

	if block.View <= v.currentView {
		return
	}

	v.currentView = block.View

	callbacks, ok := v.views[block.View]
	if !ok {
		return
	}

	for _, callback := range callbacks {
		callback(block)
	}

	// remove all invalid views
	for view := range v.views {
		if view <= v.currentView {
			delete(v.views, view)
		}
	}
}
