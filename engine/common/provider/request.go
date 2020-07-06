package provider

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Request struct {
	OriginID  flow.Identifier
	EntityIDs map[flow.Identifier]struct{}
	Timestamp time.Time
}
