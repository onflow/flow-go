package requester

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

type Item struct {
	EntityID       flow.Identifier                    // ID for the entity to be requested
	NumAttempts    uint                               // number of times the entity was requested
	LastRequested  time.Time                          // approximate timestamp of last request
	RetryAfter     time.Duration                      // interval until request should be retried
	ExtraSelector  flow.IdentityFilter[flow.Identity] // additional filters for providers of this entity
	checkIntegrity bool                               // check response integrity using `EntityID`
}
