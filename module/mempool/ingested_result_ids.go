package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// IngestedResultIDs represents a concurrency-safe memory pool for ingested execution result IDs.
// By ingested execution result IDs we mean those that have a verifiable chunk for them forwarded from
// Ingest engine to the Verify engine of Verification node
type IngestedResultIDs interface {
	// Has checks whether the mempool has the result ID
	Has(resultID flow.Identifier) bool

	// Add will add the given result ID to the memory pool or it will error if
	// the result ID is already in the memory pool.
	Add(result *flow.ExecutionResult) error

	// Rem will remove the given result ID from the memory pool; it will
	// return true if the result ID was known and removed.
	Rem(resultID flow.Identifier) bool

	// All will retrieve all result IDs that are currently in the memory pool
	// as an IdentityList
	All() flow.IdentifierList
}
