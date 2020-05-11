package integration_test

import (
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
)

type Hub struct {
	networks map[flow.Identifier]*Network
	filter   BlockOrDelayFunc
}

func NewHub() *Hub {
	h := &Hub{
		networks: make(map[flow.Identifier]*Network),
	}
	return h
}

func (h *Hub) WithFilter(filter BlockOrDelayFunc) *Hub {
	h.filter = filter
	return h
}

func (h *Hub) AddNetwork(originID flow.Identifier, node *Node) *Network {
	net := &Network{
		hub:      h,
		originID: originID,
		conduits: make(map[uint8]*Conduit),
		node:     node,
	}
	h.networks[originID] = net
	return net
}

type Network struct {
	hub      *Hub
	node     *Node
	originID flow.Identifier
	conduits map[uint8]*Conduit
}

func (n *Network) Register(channelID uint8, engine network.Engine) (network.Conduit, error) {
	con := &Conduit{
		net:       n,
		channelID: channelID,
		queue:     make(chan message, 128),
	}
	go func() {
		for msg := range con.queue {
			engine.Submit(msg.originID, msg.event)
		}
	}()
	n.conduits[channelID] = con
	return con, nil
}

type Conduit struct {
	net       *Network
	channelID uint8
	queue     chan message
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	for _, targetID := range targetIDs {
		net, found := c.net.hub.networks[targetID]
		if !found {
			return fmt.Errorf("invalid network (target: %x)", targetID)
		}
		con, found := net.conduits[c.channelID]
		if !found {
			return fmt.Errorf("invalid conduit (target: %x, channel: %d)", targetID, c.channelID)
		}

		sender, receiver := c.net.node, net.node

		block, delay := c.net.hub.filter(c.channelID, event, sender, receiver)
		// block the message
		if block {
			continue
		}

		// no delay, push to the receiver's message queue right away
		if delay == 0 {
			con.queue <- message{originID: c.net.originID, event: event}
			continue
		}

		// use a goroutine to wait and send
		go func(delay time.Duration, senderID flow.Identifier, receiver *Conduit, event interface{}) {
			// sleep in order to simulate the network delay
			time.Sleep(delay)
			con.queue <- message{originID: senderID, event: event}
		}(delay, c.net.originID, con, event)
	}
	return nil
}

type message struct {
	originID flow.Identifier
	event    interface{}
}
