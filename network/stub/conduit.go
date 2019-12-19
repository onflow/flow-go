package stub

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/network/trickle"
)

type Conduit struct {
	engineID uint8
	submit   trickle.SubmitFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...model.Identifier) error {
	return c.submit(c.engineID, event, targetIDs...)
}
