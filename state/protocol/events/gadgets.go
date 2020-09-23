package events

// Heights enables subscribing to finalized heights.
type Heights interface {

	// OnHeight registers the callback for the given height.
	OnHeight(height uint64, callback func())
}
