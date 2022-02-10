package stub

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type Conduit struct {
	channel   network.Channel
	ctx       context.Context
	cancel    context.CancelFunc
	publish   network.PublishFunc
	unicast   network.UnicastFunc
	multicast network.MulticastFunc
	close     network.CloseFunc
}

func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.publish(c.channel, event, targetIDs...)
}

func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.unicast(c.channel, event, targetID)
}

func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.multicast(c.channel, event, num, targetIDs...)
}

func (c *Conduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	c.cancel()
	return c.close(c.channel)
}
