package corruptible

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// Conduit implements a corruptible conduit that sends all incoming events to its registered controller (i.e., factory)
// without dispatching them to the networking layer.
type Conduit struct {
	ctx               context.Context
	cancel            context.CancelFunc
	channel           network.Channel
	conduitController insecure.ConduitController
}

// Publish sends the incoming events as publish events to the controller of this conduit (i.e., its factory) to handle.
func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	err := c.conduitController.HandleIncomingEvent(event, c.channel, insecure.Protocol_PUBLISH, 0, targetIDs...)
	if err != nil {
		return fmt.Errorf("factory could not handle the publish event: %w", err)
	}

	return nil
}

// Unicast sends the incoming events as unicast events to the controller of this conduit (i.e., its factory) to handle.
func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	err := c.conduitController.HandleIncomingEvent(event, c.channel, insecure.Protocol_UNICAST, 0, targetID)
	if err != nil {
		return fmt.Errorf("factory could not handle the unicast event: %w", err)
	}

	return nil
}

// Multicast sends the incoming events as multicast events to the controller of this conduit (i.e., its factory) to handle.
func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	err := c.conduitController.HandleIncomingEvent(event, c.channel, insecure.Protocol_MULTICAST, uint32(num), targetIDs...)
	if err != nil {
		return fmt.Errorf("factory could not handle the multicast event: %w", err)
	}

	return nil
}

// Close informs the conduit controller that the engine is not going to use this conduit anymore.
func (c *Conduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s already closed", c.channel)
	}
	// close the conduit context
	c.cancel()
	// call the close function
	return c.conduitController.EngineClosingChannel(c.channel)
}
