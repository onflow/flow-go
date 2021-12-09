package integration_test

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
)

// TODO replace this type with `network/stub/hub.go`
// Hub is a test helper that mocks a network overlay.
// It maintains a set of network instances and enables them to directly exchange message
// over the memory.
type Hub struct {
	networks   map[flow.Identifier]*Network
	filter     BlockOrDelayFunc
	identities flow.IdentityList
}

// NewNetworkHub creates and returns a new Hub instance.
func NewNetworkHub() *Hub {
	return &Hub{
		networks:   make(map[flow.Identifier]*Network),
		identities: flow.IdentityList{},
	}
}

// WithFilter is an option method that sets filter of the Hub instance.
func (h *Hub) WithFilter(filter BlockOrDelayFunc) *Hub {
	h.filter = filter
	return h
}

// AddNetwork stores the reference of the Network in the Hub, in order for networks to find
// other networks to send events directly.
func (h *Hub) AddNetwork(originID flow.Identifier, node *Node) *Network {
	net := &Network{
		ctx:      context.Background(),
		hub:      h,
		originID: originID,
		conduits: make(map[network.Channel]*Conduit),
		node:     node,
	}
	h.networks[originID] = net
	h.identities = append(h.identities, node.id)
	return net
}

// TODO replace this type with `network/stub/network.go`
// Network is a mocked Network layer made for testing engine's behavior.
// It represents the Network layer of a single node. A node can attach several engines of
// itself to the Network, and hence enabling them send and receive message.
// When an engine is attached on a Network instance, the mocked Network delivers
// all engine's events to others using an in-memory delivery mechanism.
type Network struct {
	ctx      context.Context
	hub      *Hub
	node     *Node
	originID flow.Identifier
	conduits map[network.Channel]*Conduit
	mocknetwork.Network
}

// Register registers an Engine of the attached node to the channel via a Conduit, and returns the
// Conduit instance.
func (n *Network) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	ctx, cancel := context.WithCancel(n.ctx)
	con := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		net:     n,
		channel: channel,
		queue:   make(chan message, 1024),
	}
	go func() {
		for msg := range con.queue {
			engine.Submit(channel, msg.originID, msg.event)
		}
	}()
	n.conduits[channel] = con
	return con, nil
}

// unregister unregisters the engine associated with the given channel and closes the conduit queue.
func (n *Network) unregister(channel network.Channel) error {
	con := n.conduits[channel]
	close(con.queue)
	delete(n.conduits, channel)
	return nil
}

// submit is called when the attached Engine to the channel is sending an event to an
// Engine attached to the same channel on another node or nodes.
// This implementation uses unicast under the hood.
func (n *Network) submit(event interface{}, channel network.Channel, targetIDs ...flow.Identifier) error {
	var sendErrors *multierror.Error
	for _, targetID := range targetIDs {
		if err := n.unicast(event, channel, targetID); err != nil {
			sendErrors = multierror.Append(sendErrors, fmt.Errorf("could not unicast the event: %w", err))
			return fmt.Errorf("could not unicast the event: %w", err)
		}
	}
	return sendErrors.ErrorOrNil()
}

// unicast is called when the attached Engine to the channel is sending an event to a single target
// Engine attached to the same channel on another node.
func (n *Network) unicast(event interface{}, channel network.Channel, targetID flow.Identifier) error {
	net, found := n.hub.networks[targetID]
	if !found {
		return fmt.Errorf("could not find target network on hub: %x", targetID)
	}
	con, found := net.conduits[channel]
	if !found {
		return fmt.Errorf("invalid channel (%d) for target ID (%x)", targetID, channel)
	}

	sender, receiver := n.node, net.node
	block, delay := n.hub.filter(channel, event, sender, receiver)
	// block the message
	if block {
		return nil
	}

	// no delay, push to the receiver's message queue right away
	if delay == 0 {
		con.queue <- message{originID: n.originID, event: event}
		return nil
	}

	// use a goroutine to wait and send
	go func(delay time.Duration, senderID flow.Identifier, receiver *Conduit, event interface{}) {
		// sleep in order to simulate the network delay
		time.Sleep(delay)
		con.queue <- message{originID: senderID, event: event}
	}(delay, n.originID, con, event)

	return nil
}

// publish is called when the attached Engine is sending an event to a group of Engines attached to the
// same channel on other nodes based on selector.
// In this test helper implementation, publish uses submit method under the hood.
func (n *Network) publish(event interface{}, channel network.Channel, targetIDs ...flow.Identifier) error {
	return n.submit(event, channel, targetIDs...)
}

// multicast is called when an Engine attached to the channel is sending an event to a number of randomly chosen
// Engines attached to the same channel on other nodes. The targeted nodes are selected based on the selector.
// In this test helper implementation, multicast uses submit method under the hood.
func (n *Network) multicast(event interface{}, channel network.Channel, num uint, targetIDs ...flow.Identifier) error {
	targetIDs = flow.Sample(num, targetIDs...)
	return n.submit(event, channel, targetIDs...)
}

type Conduit struct {
	ctx     context.Context
	cancel  context.CancelFunc
	net     *Network
	channel network.Channel
	queue   chan message
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit closed")
	}
	return c.net.submit(event, c.channel, targetIDs...)
}

func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit closed")
	}
	return c.net.publish(event, c.channel, targetIDs...)
}

func (c *Conduit) Unicast(event interface{}, targetID flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit closed")
	}
	return c.net.unicast(event, c.channel, targetID)
}

func (c *Conduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit closed")
	}
	return c.net.multicast(event, c.channel, num, targetIDs...)
}

func (c *Conduit) Close() error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit closed")
	}
	c.cancel()
	return c.net.unregister(c.channel)
}

type message struct {
	originID flow.Identifier
	event    interface{}
}
