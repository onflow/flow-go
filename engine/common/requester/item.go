package requester

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

type Request struct {
	QueryKey           flow.Identifier                    // the key used to identify the requested entity (content hash or secondary key)
	NumAttempts        uint                               // number of times the entity was requested
	LastRequested      time.Time                          // approximate timestamp of last request
	RetryAfter         time.Duration                      // interval until request should be retried
	ExtraSelector      flow.IdentityFilter[flow.Identity] // additional filters for providers of this entity
	queryByContentHash bool                               // whether QueryKey is the content hash of the requested entity
}
