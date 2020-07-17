package stub

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

type Conduit struct {
	channelID uint8
	send      libp2p.SendFunc
}

func (c *Conduit) Send(selection network.SelectionFunc, selector flow.IdentityFilter, message interface{}) error {
	return c.send(c.channelID, selection, selector, message)
}
