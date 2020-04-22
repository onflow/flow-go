// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

// Views is used by HotStuff to persist certain views in order to
// maintain the correct state between crashes.
type Views interface {

	// Store stores the latest view used by hotstuff.
	Store(action uint8, view uint64) error

	// Retrieve retrieves the latest view used by hotstuff.
	Retrieve(action uint8) (uint64, error)
}
