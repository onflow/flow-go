// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package adaptor

import (
	"context"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/registry"
)

// Network implements the network interface as a wrapper around the gossip
// package.
type Network struct {
	node    *gossip.Node
	codec   network.Codec
	me      module.Local
	engines map[uint8]network.Engine
}

// NewNetwork creates a new implementation of the network interface, wrapping
// around the provided gossip node and using the given codec.
func NewNetwork(node *gossip.Node, codec network.Codec, me module.Local) (*Network, error) {

	w := Network{
		node:    node,
		codec:   codec,
		me:      me,
		engines: make(map[uint8]network.Engine),
	}

	return &w, nil
}

// Register will register a new engine with the wrapping adaptor. The returned
// conduit will use the GRPC functionality of the underlying gossip node to
// create a network bus for each engine.
func (n *Network) Register(engineID uint8, engine network.Engine) (network.Conduit, error) {

	// check if the engine slot is still free
	_, ok := n.engines[engineID]
	if ok {
		return nil, errors.Errorf("engine already registered (%d)", engineID)
	}

	// create and register sender for receiving
	conduit := &Conduit{
		engineID: engineID,
		submit:   n.submit,
		handle:   n.handle,
	}
	msgType := registry.MessageType(engineID)
	err := n.node.RegisterFunc(msgType, conduit.Handle)
	if err != nil {
		return nil, errors.Wrap(err, "could not register handler")
	}

	// NOTE: as the current gossip protocol does not include requesting of
	// entities (which thus is entirely handled on the application layer), the
	// requester here is never used

	// register the engine
	n.engines[engineID] = engine

	return conduit, nil
}

func (n *Network) submit(engineID uint8, event interface{}, recipients ...string) error {

	// encode the event
	payload, err := n.codec.Encode(event)
	if err != nil {
		return errors.Wrap(err, "could not encode event")
	}

	// gossip the message using the engine code as message type
	msgType := registry.MessageType(engineID)
	_, err = n.node.Gossip(context.Background(), payload, recipients, msgType)
	if err != nil {
		return errors.Wrap(err, "could not gossip event")
	}

	return nil
}

func (n *Network) handle(engineID uint8, payload []byte) error {

	// check if we have the given engine receiver registered
	engine, ok := n.engines[engineID]
	if !ok {
		return errors.Errorf("could not find engine (%d)", engine)
	}

	// decode the payload
	event, err := n.codec.Decode(payload)
	if err != nil {
		return errors.Wrap(err, "could not decode event")
	}

	// NOTE: current gossip implementation does not forward node ID, so we have
	// no idea where we received the payload from; we simply pass an empty string

	// bubble up payload to engine
	err = engine.Process(n.me.NodeID(), event)
	if err != nil {
		return errors.Wrap(err, "could not receive payload")
	}

	return nil
}
