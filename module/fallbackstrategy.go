package module

// FallbackStrategy defines an interface to handle usage of multiple
// clients connected to different hosts.
type FallbackStrategy interface {
	// ClientIndex return index of client to use
	ClientIndex() int

	// Failure indicate failure to use client at active index
	Failure()
}
