package corruptible

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	ctx      context.Context
	adapter  network.Adapter
	cancel   context.CancelFunc
	channel  network.Channel
	observer insecure.Observer
}

// Publish implements a corruptible publishing service that is it sends all incoming messages
// directly to the conduit factory to decide what to do next.
// If the conduit factory gracefully rejects to observe the dispatched message,
// Publish acts as in the non-corrupted mode and behaves correctly as expected.
// which is sending the event to the network layer for unreliable delivery
// to subscribers of the given event on the network layer. It uses a
// publish-subscribe layer and can thus not guarantee that the specified
// recipients received the event.
func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	observed, err := c.observer.Observe(c.ctx, event, c.channel, proto.Protocol_PUBLISH, 0, targetIDs...)
	if err != nil {
		return fmt.Errorf("factory could not observe the publish event: %w", err)
	}

	if !observed {
		// if corruptible conduit factor gracefully decides not to observe a message, the
		// normal flow of passing the message to the networking layer should be followed.
		return c.adapter.PublishOnChannel(c.channel, event, targetIDs...)
	}

	return nil
}

// Unicast implements a corruptible unicasting service that is it sends all incoming messages
// directly to the conduit factory to decide what to do next.
// If the conduit factory gracefully rejects to observe the dispatched message,
// Unicast acts as in the non-corrupted mode and behaves correctly as expected, which is
// sending an event in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the event.
// It returns an error if the unicast fails.
func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}

	observed, err := c.observer.Observe(c.ctx, event, c.channel, proto.Protocol_UNICAST, 0, targetID)
	if err != nil {
		return fmt.Errorf("factory could not observe the unicast event: %w", err)
	}

	if !observed {
		// if corruptible conduit factor gracefully decides not to observe a message, the
		// normal flow of passing the message to the networking layer should be followed.
		return c.adapter.UnicastOnChannel(c.channel, event, targetID)
	}

	return nil
}

// Multicast unreliably sends the specified event to the specified number of recipients selected from the specified subset.
// The recipients are selected randomly from targetIDs
func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.adapter.MulticastOnChannel(c.channel, event, num, targetIDs...)
}

func (c *Conduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s already closed", c.channel)
	}
	// close the conduit context
	c.cancel()
	// call the close function
	return c.adapter.UnRegisterChannel(c.channel)
}
