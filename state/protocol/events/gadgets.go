package events

// Heights enables subscribing to specific heights. The callback is invoked
// when the given height is finalized.
type Heights interface {

	// OnHeight registers the callback for the given height.
	OnHeight(height uint64, callback func())
}
