package messages

// UntrustedMessage is the decode target for anything that came over the wire.
// ToInternal must validate and construct the internal, trusted model.
// Implementations must validate and construct the trusted, internal model.
type UntrustedMessage interface {

	// ToInternal returns the validated internal type (from flow.* constructors)
	ToInternal() (any, error)
}
