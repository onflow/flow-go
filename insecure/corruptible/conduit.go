package corruptible

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// SlaveConduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type SlaveConduit struct {
	ctx     context.Context
	cancel  context.CancelFunc
	channel network.Channel
	master  insecure.ConduitMaster
}

// Publish sends the incoming events as publish events to the master of this conduit (i.e., its factory) to handle.
func (c *SlaveConduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	err := c.master.HandleIncomingEvent(c.ctx, event, c.channel, proto.Protocol_PUBLISH, 0, targetIDs...)
	if err != nil {
		return fmt.Errorf("factory could not handle the publish event: %w", err)
	}

	return nil
}

// Unicast sends the incoming events as unicast events to the master of this conduit (i.e., its factory) to handle.
func (c *SlaveConduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	err := c.master.HandleIncomingEvent(c.ctx, event, c.channel, proto.Protocol_UNICAST, 0, targetID)
	if err != nil {
		return fmt.Errorf("factory could not handle the unicast event: %w", err)
	}

	return nil
}

// Multicast sends the incoming events as multicast events to the master of this conduit (i.e., its factory) to handle.
func (c *SlaveConduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	err := c.master.HandleIncomingEvent(c.ctx, event, c.channel, proto.Protocol_MULTICAST, uint32(num), targetIDs...)
	if err != nil {
		return fmt.Errorf("factory could not handle the multicast event: %w", err)
	}

	return nil
}

// Close informs the
func (c *SlaveConduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s already closed", c.channel)
	}
	// close the conduit context
	c.cancel()
	// call the close function
	return c.master.EngineIsDoneWithMe(c.channel)
}
