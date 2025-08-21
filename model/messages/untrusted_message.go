package messages

// UntrustedMessage represents the set of allowed decode target types for messages received over the network.
// Conceptually, an UntrustedMessage implementation makes no guarantees whatsoever about its contents.
// UntrustedMessage's must implement a ToInternal method, which converts the network message to a corresponding internal type (typically in the model/flow package)
// This conversion provides an opportunity to:
//   - perform basic structural validity checks (required fields are non-nil, fields reference one another in a consistent way)
//   - attach auxiliary information (for example, caching the hash of a model)
//
// Internal models abide by basic structural validity requirements, but are not trusted.
// They may still represent invalid or Byzantine inputs in the context of the broader application state.
// It is the responsibility of engines operating on these models to fully validate them.
type UntrustedMessage interface {

	// ToInternal returns the internal type (from flow.* constructors) representation.
	// All errors indicate that the decode target contains a structurally invalid representation of the internal model.
	ToInternal() (any, error)
}
