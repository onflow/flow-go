package storage

import "github.com/onflow/flow-go/ledger"

// Registers represent persistent storage for execution data registers.
type Registers interface {

	// Store insert register payload indexed by path. If update paths exist the payloads are overwritten.
	Store(path ledger.Path, payload *ledger.Payload) error

	// ByPath returns trie update payloads for provided path.
	ByPath(paths ledger.Path) (*ledger.Payload, error)

	// TODO(sideninja) maybe we should add batch insert and retrieve methods to optimize operations.
}
