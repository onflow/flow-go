package notifications

// Consumer defines an interface for consuming informational
// notifications about the operation of the core HotStuff algorithm.
type Consumer interface {

	// Consume consumes a notification event from HotStuff. The set of possible
	// notifications is specified as types in (TODO define notification types).
	Consume(notification interface{})
}
