package gossip

import (
	"github.com/dapperlabs/flow-go/model"
)

// sendFunc type is used for conduit
type sendFunc func(event interface{}, targetIDs ...model.Identifier) error

// conduit to be passed to engine registries
type conduit struct {
	send sendFunc
}

// Submit satisfies the conduit interface and enables message delivery between
// engines
func (c *conduit) Submit(event interface{}, targetIDs ...model.Identifier) error {
	return c.send(event, targetIDs...)
}
