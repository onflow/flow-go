package requester

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Batch struct {
	Nonce     uint64
	Timestamp time.Time
	Retry     time.Duration
	TargetID  flow.Identifier
	EntityIDs []flow.Identifier
}
