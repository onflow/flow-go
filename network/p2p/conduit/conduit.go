package conduit

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

// DefaultConduitFactory is a wrapper around the network ConduitAdapter.
// It directly passes the incoming messages to the corresponding methods of the
// network ConduitAdapter.
type DefaultConduitFactory struct {
	adapter network.ConduitAdapter
}

var _ network.ConduitFactory = (*DefaultConduitFactory)(nil)

// NewDefaultConduitFactory creates a new DefaultConduitFactory, this is the default conduit factory used by the node.
// Args:
//
//	none
//
// Returns:
//
//	a new instance of the DefaultConduitFactory.
func NewDefaultConduitFactory() *DefaultConduitFactory {
	return &DefaultConduitFactory{}
}

// RegisterAdapter sets the ConduitAdapter component of the factory.
// The ConduitAdapter is a wrapper around the Network layer that only exposes the set of methods
// that are needed by a conduit.
func (d *DefaultConduitFactory) RegisterAdapter(adapter network.ConduitAdapter) error {
	if d.adapter != nil {
		return fmt.Errorf("could not register a new network adapter, one already exists")
	}

	d.adapter = adapter

	return nil
}

// NewConduit creates a conduit on the specified channel.
// Prior to creating any conduit, the factory requires an ConduitAdapter to be registered with it.
func (d *DefaultConduitFactory) NewConduit(ctx context.Context, channel channels.Channel) (network.Conduit, error) {
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
	channel channels.Channel
	adapter network.ConduitAdapter
}

var _ network.Conduit = (*Conduit)(nil)

// Publish sends an event to the network layer for unreliable delivery
// to subscribers of the given event on the network layer. It uses a
// publish-subscribe layer and can thus not guarantee that the specified
// recipients received the event.
func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.adapter.PublishOnChannel(c.channel, event, targetIDs...)
}

// Unicast sends an event in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the event.
// It returns an error if the unicast fails.
func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.adapter.UnicastOnChannel(c.channel, event, targetID)
}

// Multicast unreliably sends the specified event to the specified number of recipients selected from the specified subset.
// The recipients are selected randomly from targetIDs
func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit for channel %s closed", c.channel)
	}
	return c.adapter.MulticastOnChannel(c.channel, event, num, targetIDs...)
}

// ReportMisbehavior reports the misbehavior of a node on sending a message to the current node that appears valid
// based on the networking layer but is considered invalid by the current node based on the Flow protocol.
// The misbehavior is reported to the networking layer to penalize the misbehaving node.
// The implementation must be thread-safe and non-blocking.
func (c *Conduit) ReportMisbehavior(report network.MisbehaviorReport) {
	c.adapter.ReportMisbehaviorOnChannel(c.channel, report)
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
