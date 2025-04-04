package stdmap

import (
	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// Assignments implements the chunk assignment memory pool.
// Stored assignments are keyed by assignment fingerprint.
type Assignments struct {
	*Backend[flow.Identifier, *chunkmodels.Assignment]
}

// NewAssignments creates a new memory pool for Assignments.
func NewAssignments(limit uint) *Assignments {
	return &Assignments{NewBackend(WithLimit[flow.Identifier, *chunkmodels.Assignment](limit))}
}
