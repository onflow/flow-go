package events

import "github.com/onflow/flow-go/model/flow"

// Heights enables subscribing to specific heights. The callback is invoked
// when the given height is finalized.
type Heights interface {

	// OnHeight registers the callback for the given height.
	OnHeight(height uint64, callback func())
}

// OnViewCallback is the type of callback triggered by view events.
type OnViewCallback func(*flow.Header)

// Views enables subscribing to specific views. The callback is invoked when the
// first block of the given view is finalized.
type Views interface {
	// OnView registers the callback for the given view.
	OnView(view uint64, callback OnViewCallback)
}
