package p2p

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// PublishFunc is a function that broadcasts the specified event
// to all participants on the given channel.
type PublishFunc func(channel network.Channel, event interface{}, targetIDs ...flow.Identifier) error

// UnicastFunc is a function that reliably sends the event via reliable 1-1 direct
// connection in  the underlying network to the target ID.
type UnicastFunc func(channel network.Channel, event interface{}, targetID flow.Identifier) error

// MulticastFunc is a function that unreliably sends the event in the underlying
// network to randomly chosen subset of nodes from targetIDs
type MulticastFunc func(channel network.Channel, event interface{}, num uint, targetIDs ...flow.Identifier) error

// CloseFunc is a function that unsubscribes the conduit from the channel
type CloseFunc func(channel network.Channel) error

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	ctx       context.Context
	cancel    context.CancelFunc
	channel   network.Channel
	publish   PublishFunc
	unicast   UnicastFunc
	multicast MulticastFunc
	close     CloseFunc
}

// Publish sends an event to the network layer for unreliable delivery
// to subscribers of the given event on the network layer. It uses a
// publish-subscribe layer and can thus not guarantee that the specified
// recipients received the event.
func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.publish(c.channel, event, targetIDs...)
}

// Unicast sends an event in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the event.
// It returns an error if the unicast fails.
func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.unicast(c.channel, event, targetID)
}

// Multicast unreliably sends the specified event to the specified number of recipients selected from the specified subset.
// The recipients are selected randomly from targetIDs
func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.multicast(c.channel, event, num, targetIDs...)
}

func (c *Conduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s already closed", c.channel)
	}
	// close the conduit context
	c.cancel()
	// call the close function
	return c.close(c.channel)
}
