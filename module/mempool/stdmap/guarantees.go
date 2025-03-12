package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees implements the collections memory pool of the consensus nodes,
// used to store collection guarantees and to generate block payloads.
// Stored Collection Guarantees are keyed by collection ID.
type Guarantees struct {
	*Backend[flow.Identifier, *flow.CollectionGuarantee]
}

// NewGuarantees creates a new memory pool for collection guarantees.
func NewGuarantees(limit uint) (*Guarantees, error) {
	return &Guarantees{NewBackend(WithLimit[flow.Identifier, *flow.CollectionGuarantee](limit))}, nil
}
