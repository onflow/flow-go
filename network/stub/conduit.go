package stub

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p"
)

type Conduit struct {
	channelID string
	ctx       context.Context
	cancel    context.CancelFunc
	submit    libp2p.SubmitFunc
	publish   libp2p.PublishFunc
	unicast   libp2p.UnicastFunc
	multicast libp2p.MulticastFunc
	close     libp2p.CloseFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel ID %s closed", c.channelID)
	}
	return c.submit(c.channelID, event, targetIDs...)
}

func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel ID %s closed", c.channelID)
	}
	return c.publish(c.channelID, event, targetIDs...)
}

func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel ID %s closed", c.channelID)
	}
	return c.unicast(c.channelID, event, targetID)
}

func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel ID %s closed", c.channelID)
	}
	return c.multicast(c.channelID, event, num, targetIDs...)
}

func (c *Conduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel ID %s closed", c.channelID)
	}
	c.cancel()
	return c.close(c.channelID)
}
