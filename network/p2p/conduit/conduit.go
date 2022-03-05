package conduit

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/network"
)

// DefaultConduitFactory is a wrapper around the network Adapter.
// It directly passes the incoming messages to the corresponding methods of the
// network Adapter.
type DefaultConduitFactory struct {
	adapter network.Adapter
}

func NewDefaultConduitFactory() *DefaultConduitFactory {
	return &DefaultConduitFactory{}
}

// RegisterAdapter sets the Adapter component of the factory.
// The Adapter is a wrapper around the Network layer that only exposes the set of methods
// that are needed by a conduit.
func (d *DefaultConduitFactory) RegisterAdapter(adapter network.Adapter) error {
	if d.adapter != nil {
		return fmt.Errorf("could not register a new network adapter, one already exists")
	}

	d.adapter = adapter

	return nil
}

// NewConduit creates a conduit on the specified channel.
// Prior to creating any conduit, the factory requires an Adapter to be registered with it.
func (d *DefaultConduitFactory) NewConduit(ctx context.Context, channel network.Channel) (network.Conduit, error) {
	if d.adapter == nil {
		return nil, fmt.Errorf("could not create a new conduit, missing a registered network adapter")
	}

	child, cancel := context.WithCancel(ctx)

	return &Conduit{
		ctx:     child,
		cancel:  cancel,
		channel: channel,
		adapter: d.adapter,
	}, nil
}

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	ctx     context.Context
	cancel  context.CancelFunc
	channel network.Channel
	adapter network.Adapter
}

// Publish sends an event to the network layer for unreliable delivery
// to subscribers of the given event on the network layer. It uses a
// publish-subscribe layer and can thus not guarantee that the specified
// recipients received the event.
func (c *Conduit) Publish(event interface{}) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.adapter.PublishOnChannel(c.channel, event)
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
