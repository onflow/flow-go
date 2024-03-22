package protocol_state

// ViewBasedActivator allows setting value that will be active from specific view.
type ViewBasedActivator[T any] struct {
	Data           T
	ActivationView uint64 // first view at which Data will take effect
}

// VersionedEncodable defines the interface for a versioned key-value store independent
// of the set of keys which are supported. This allows the storage layer to support
// storing different key-value model versions within the same software version.
type VersionedEncodable interface {
	// VersionedEncode encodes the key-value store, returning the version separately
	// from the encoded bytes.
	// No errors are expected during normal operation.
	VersionedEncode() (uint64, []byte, error)
}
