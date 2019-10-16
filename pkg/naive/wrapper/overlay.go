// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package wrapper

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/codec"
	"github.com/dapperlabs/flow-go/pkg/naive"
	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
)

// Overlay implements a wrapper around the gossip package that implements our
// simple overlay interface.
type Overlay struct {
	node    *gnode.Node
	codec   codec.Codec
	engines map[uint8]naive.Engine
}

func New(node *gnode.Node, codec codec.Codec) (*Overlay, error) {

	w := Overlay{
		node:    node,
		codec:   codec,
		engines: make(map[uint8]naive.Engine),
	}

	return &w, nil
}

func (o *Overlay) Register(code uint8, engine naive.Engine) (naive.Conduit, error) {

	// check if the engine slot is still free
	_, ok := o.engines[code]
	if ok {
		return nil, errors.Errorf("engine already registered (%d)", code)
	}

	// create and register sender for receiving
	conduit := &Conduit{
		code: code,
		send: o.send,
		recv: o.recv,
	}
	msgType := fmt.Sprint(code)
	err := o.node.RegisterFunc(msgType, conduit.Handle)
	if err != nil {
		return nil, errors.Wrap(err, "could not register handler")
	}

	// NOTE: as the current gossip protocol does not include requesting of
	// entities (which thus is entirely handled on the application layer), the
	// requester here is never used

	// register the engine
	o.engines[code] = engine

	return conduit, nil
}

func (o *Overlay) send(code uint8, event interface{}, recipients ...string) error {

	// encode the event
	payload, err := o.codec.Encode(event)
	if err != nil {
		return errors.Wrap(err, "could not encode event")
	}

	// gossip the message using the engine code as message type
	msgType := fmt.Sprint(code)
	_, err = o.node.AsyncGossip(context.Background(), payload, recipients, msgType)
	if err != nil {
		return errors.Wrap(err, "could not gossip event")
	}

	return nil
}

func (o *Overlay) recv(code uint8, payload []byte) error {

	// check if we have the given engine receiver registered
	engine, ok := o.engines[code]
	if !ok {
		return errors.Errorf("could not find engine (%d)", engine)
	}

	// decode the payload
	event, err := o.codec.Decode(payload)
	if err != nil {
		return errors.Wrap(err, "could not decode event")
	}

	// NOTE: current gossip implementation does not forward node ID, so we have
	// no idea where we received the payload from; we simply pass an empty string

	// bubble up payload to engine
	err = engine.Receive("", event)
	if err != nil {
		return errors.Wrap(err, "could not receive payload")
	}

	return nil
}
