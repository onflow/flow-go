// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

type Views interface {

	// StoreLatest stores the latest view used by hotstuff.
	StoreLatest(view uint64) error

	// RetrieveLatest retrieves the latest view used by hotstuff.
	RetrieveLatest() (uint64, error)
}
