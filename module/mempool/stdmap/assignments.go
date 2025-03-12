package stdmap

import (
	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// Assignments implements the chunk assignment memory pool.
// Stored Assignments are keyed by assignment fingerprint.
type Assignments struct {
	*Backend[flow.Identifier, *chunkmodels.Assignment]
}

// NewAssignments creates a new memory pool for Assignments.
func NewAssignments(limit uint) (*Assignments, error) {
	return &Assignments{NewBackend(WithLimit[flow.Identifier, *chunkmodels.Assignment](limit))}, nil
}
