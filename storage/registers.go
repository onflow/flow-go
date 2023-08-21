package storage

import "github.com/onflow/flow-go/ledger"

// Registers represent persistent storage for execution data registers.
type Registers interface {

	// Store inserts trie updates with provided paths and payloads. Duplicate is ignored.
	Store(update *ledger.TrieUpdate) error

	// ByPaths returns trie update payloads for provided paths.
	ByPaths(paths []ledger.Path) ([]*ledger.Payload, error)
}
