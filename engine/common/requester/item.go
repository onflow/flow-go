package requester

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Item struct {
	EntityID  flow.Identifier
	Attempts  uint
	Timestamp time.Time
	Interval  time.Duration
}
