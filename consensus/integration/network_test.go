package integration_test

import (
	"context"
	"fmt"
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
		ctx:                   context.Background(),
		hub:                   h,
		originID:              originID,
		conduits:              make(map[network.Channel]*Conduit),
		directMessageHandlers: make(map[network.Channel]network.DirectMessageHandler),
		directMessageQueues:   make(map[network.Channel]chan message),
		node:                  node,
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
	ctx                   context.Context
	hub                   *Hub
	node                  *Node
	originID              flow.Identifier
	conduits              map[network.Channel]*Conduit
	directMessageHandlers map[network.Channel]network.DirectMessageHandler
	directMessageQueues   map[network.Channel]chan message
	mocknetwork.Network
}

func (n *Network) RegisterDirectMessageHandler(channel network.Channel, handler network.DirectMessageHandler) error {
	queue := make(chan message, 256)

	go func() {
		for msg := range queue {
			go func(m message) {
				handler(channel, m.originID, m.event)
			}(msg)
		}
	}()

	n.directMessageHandlers[channel] = handler
	n.directMessageQueues[channel] = queue

	return nil
}

func (n *Network) SendDirectMessage(channel network.Channel, event interface{}, targetID flow.Identifier) error {
	targetNet, ok := n.hub.networks[targetID]
	if !ok {
		return fmt.Errorf("target node %s not found", targetID)
	}

	queue, ok := targetNet.directMessageQueues[channel]
	if !ok {
		return fmt.Errorf("no direct message handler registered for channel %s", channel)
	}

	sender, receiver := n.node, targetNet.node
	block, delay := n.hub.filter(channel, event, sender, receiver)
	// block the message
	if block {
		return nil
	}

	// no delay, push to the receiver's message queue right away
	if delay == 0 {
		queue <- message{originID: n.originID, event: event}
		return nil
	}

	// use a goroutine to wait and send
	go func(delay time.Duration, senderID flow.Identifier, receiver chan<- message, event interface{}) {
		// sleep in order to simulate the network delay
		time.Sleep(delay)
		queue <- message{originID: senderID, event: event}
	}(delay, n.originID, queue, event)

	return nil
}

// Register registers an Engine of the attached node to the channel via a Conduit, and returns the
// Conduit instance.
func (n *Network) Register(channel network.Channel, engine network.MessageProcessor) (network.Conduit, error) {
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
			go func(m message) {
				_ = engine.Process(channel, m.originID, m.event)
			}(msg)
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

// trySend is called when the attached Engine to the channel is sending an event to a single target
// Engine attached to the same channel on another node.
func (n *Network) trySend(event interface{}, channel network.Channel, net *Network) {
	con, found := net.conduits[channel]
	if !found {
		return
	}

	sender, receiver := n.node, net.node
	block, delay := n.hub.filter(channel, event, sender, receiver)
	// block the message
	if block {
		return
	}

	// no delay, push to the receiver's message queue right away
	if delay == 0 {
		con.queue <- message{originID: n.originID, event: event}
		return
	}

	// use a goroutine to wait and send
	go func(delay time.Duration, senderID flow.Identifier, receiver *Conduit, event interface{}) {
		// sleep in order to simulate the network delay
		time.Sleep(delay)
		con.queue <- message{originID: senderID, event: event}
	}(delay, n.originID, con, event)
}

// publish is called when the attached Engine is sending an event to a group of Engines attached to the
// same channel on other nodes based on selector.
func (n *Network) publish(event interface{}, channel network.Channel) {
	for targetID, net := range n.hub.networks {
		if targetID != n.originID {
			n.trySend(event, channel, net)
		}
	}
}

type Conduit struct {
	ctx     context.Context
	cancel  context.CancelFunc
	net     *Network
	channel network.Channel
	queue   chan message
}

func (c *Conduit) Publish(event interface{}) error {
	if c.ctx.Err() != nil {
		return fmt.Errorf("conduit closed")
	}
	c.net.publish(event, c.channel)
	return nil
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
