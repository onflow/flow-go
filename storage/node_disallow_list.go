package storage

import "github.com/onflow/flow-go/model/flow"

// NodeDisallowList represents persistent storage for node disallow list.
type NodeDisallowList interface {
	// Store writes the given disallowList to the database.
	// To avoid legacy entries in the database, we purge
	// the entire database entry if disallowList is empty.
	// No errors are expected during normal operations.
	Store(disallowList map[flow.Identifier]struct{}) error

	// Retrieve reads the set of disallowed nodes from the database.
	// No error is returned if no database entry exists.
	// No errors are expected during normal operations.
	Retrieve(disallowList *map[flow.Identifier]struct{}) error
}
