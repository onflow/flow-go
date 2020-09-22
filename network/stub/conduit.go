package stub

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

type Conduit struct {
	channelID string
	submit    libp2p.SubmitFunc
	publish   libp2p.PublishFunc
	unicast   libp2p.UnicastFunc
	multicast libp2p.MulticastFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return c.submit(c.channelID, event, targetIDs...)
}

func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	return c.publish(c.channelID, event, targetIDs...)
}

func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	return c.unicast(c.channelID, event, targetID)
}

func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	return c.multicast(c.channelID, event, num, targetIDs...)
}
