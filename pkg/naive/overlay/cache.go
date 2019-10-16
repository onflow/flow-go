// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

// Cache is responsible for caching events on the overlay layer, so that we
// don't need to look them up on the application layer for each request.
type Cache interface {
	Has(id []byte) bool
	Get(id []byte) ([]byte, bool)
	Add(id []byte, payload []byte)
}
