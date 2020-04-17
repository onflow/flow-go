// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

// Views is used by HotStuff to persist certain views in order to
// maintain the correct state between crashes.
type Views interface {

	// StoreLatest stores the latest view used by hotstuff.
	StoreLatest(view uint64) error

	// RetrieveLatest retrieves the latest view used by hotstuff.
	RetrieveLatest() (uint64, error)
}
