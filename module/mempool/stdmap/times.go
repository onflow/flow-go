package stdmap

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Times implements the times memory pool used to store time.Time values associated with
// flow.Identifiers for tracking transaction metrics in Access nodes.
type Times struct {
	*Backend[flow.Identifier, time.Time]
}

// NewTimes creates a new memory pool for times.
func NewTimes(limit uint) *Times {
	return &Times{NewBackend(WithLimit[flow.Identifier, time.Time](limit))}
}
