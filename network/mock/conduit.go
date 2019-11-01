package mock

import (
	"github.com/dapperlabs/flow-go/network/trickle"
)

type Conduit struct {
	engineID uint8
	submit   trickle.SubmitFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...string) error {
	return c.submit(c.engineID, event, targetIDs...)
}
