package core

import "time"

// Controller holds all operational parameters for the reactor.Core
// that could change dynamically over time. Implementation must be concurrency safe.
type Controller interface {
	// ToDo : can we replace this by Go's built-in context ?
	CurrentTimeout() time.Duration
}
