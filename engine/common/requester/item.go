package requester

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Item struct {
	EntityID      flow.Identifier // ID for the entity to be requested
	NumAttempts   uint            // number of times the entity was requested
	LastRequested time.Time       // approximate timestamp of last request
	RetryAfter    time.Duration   // interval until request should be retried
}
