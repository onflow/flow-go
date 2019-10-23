// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package adaptor

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/codec"
	"github.com/dapperlabs/flow-go/pkg/network"
	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
)

// Network implements the network interface as a wrapper around the gossip
// package.
type Network struct {
	node    *gnode.Node
	codec   codec.Codec
	engines map[uint8]network.Engine
}

// NewNetwork creates a new implementation of the network interface, wrapping
// around the provided gossip node and using the given codec.
func NewNetwork(node *gnode.Node, codec codec.Codec) (*Network, error) {

	w := Network{
		node:    node,
		codec:   codec,
		engines: make(map[uint8]network.Engine),
	}

	return &w, nil
}

// Register will register a new engine with the wrapping adaptor. The returned
// conduit will use the GRPC functionality of the underlying gossip node to
// create a network bus for each engine.
func (n *Network) Register(code uint8, engine network.Engine) (network.Conduit, error) {

	// check if the engine slot is still free
	_, ok := n.engines[code]
	if ok {
		return nil, errors.Errorf("engine already registered (%d)", code)
	}

	// create and register sender for receiving
	conduit := &Conduit{
		code: code,
		send: n.send,
		recv: n.recv,
	}
	msgType := fmt.Sprint(code)
	err := n.node.RegisterFunc(msgType, conduit.Handle)
	if err != nil {
		return nil, errors.Wrap(err, "could not register handler")
	}

	// NOTE: as the current gossip protocol does not include requesting of
	// entities (which thus is entirely handled on the application layer), the
	// requester here is never used

	// register the engine
	n.engines[code] = engine

	return conduit, nil
}

func (n *Network) send(code uint8, event interface{}, recipients ...string) error {

	// encode the event
	payload, err := n.codec.Encode(event)
	if err != nil {
		return errors.Wrap(err, "could not encode event")
	}

	// gossip the message using the engine code as message type
	msgType := fmt.Sprint(code)
	_, err = n.node.AsyncGossip(context.Background(), payload, recipients, msgType)
	if err != nil {
		return errors.Wrap(err, "could not gossip event")
	}

	return nil
}

func (n *Network) recv(code uint8, payload []byte) error {

	// check if we have the given engine receiver registered
	engine, ok := n.engines[code]
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
	err = engine.Process("", event)
	if err != nil {
		return errors.Wrap(err, "could not receive payload")
	}

	return nil
}
