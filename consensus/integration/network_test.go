package integration_test

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
)

type Hub struct {
	networks map[flow.Identifier]*Network
}

func NewHub() *Hub {
	h := &Hub{
		networks: make(map[flow.Identifier]*Network),
	}
	return h
}

func (h *Hub) AddNetwork(originID flow.Identifier) *Network {
	net := &Network{
		hub:      h,
		originID: originID,
		conduits: make(map[uint8]*Conduit),
	}
	h.networks[originID] = net
	return net
}

type Network struct {
	hub      *Hub
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
		con.queue <- message{originID: c.net.originID, event: event}
	}
	return nil
}

type message struct {
	originID flow.Identifier
	event    interface{}
}
