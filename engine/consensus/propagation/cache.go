// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

// Cache keeps a cache of information for the propagation protocol.
type Cache interface {
	Close()
}
