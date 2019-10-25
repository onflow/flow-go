package mock

import (
	"github.com/dapperlabs/flow-go/pkg/network/trickle"
)

type Conduit struct {
	engineID uint8
	send     trickle.SendFunc
}

func (c *Conduit) Send(event interface{}, targetIDs ...string) error {
	return c.send(c.engineID, event, targetIDs...)
}
