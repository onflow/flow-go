// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

// Views is used by HotStuff to persist certain views in order to
// maintain the correct state between crashes.
type Views interface {

	// StoreStarted stores the latest view used by hotstuff.
	StoreStarted(view uint64) error

	// RetrieveStarted retrieves the latest view used by hotstuff.
	RetrieveStarted() (uint64, error)

	// StoreVoted stores the latest view voted on.
	StoreVoted(view uint64) error

	// RetrieveVoted retrieves the latest view we voted on.
	RetrieveVoted() (uint64, error)
}
