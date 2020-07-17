package stub

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

type Conduit struct {
	channelID uint8
	transmit  libp2p.TransmitFunc
	send      libp2p.SendFunc
	publish   libp2p.PublishFunc
}

func (c *Conduit) Transmit(message interface{}, recipientIDs ...flow.Identifier) error {
	return c.transmit(c.channelID, message, recipientIDs...)
}

func (c *Conduit) Send(message interface{}, num uint, selector flow.IdentityFilter) error {
	return c.send(c.channelID, message, num, selector)
}

func (c *Conduit) Publish(message interface{}, restrictor flow.IdentityFilter) error {
	return c.publish(c.channelID, message, restrictor)
}
