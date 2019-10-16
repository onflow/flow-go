// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/pkg/codec"
	"github.com/dapperlabs/flow-go/pkg/model/hotstuff"
	"github.com/dapperlabs/flow-go/pkg/naive"
)

const (
	HotstuffEngine = 2
)

// Engine implements the hotstuff consensus engine to coordinate the consensus
// between consensus nodes on the network.
type Engine struct {
	log     zerolog.Logger
	codec   codec.Codec
	conduit naive.Conduit
}

// NewEngine creates a new hotstuff consensus engine.
func NewEngine(log zerolog.Logger, codec codec.Codec, net naive.Network) (*Engine, error) {

	// initialize the consensus engine
	e := &Engine{
		log:   log,
		codec: codec,
	}

	// register the consensus engine with the network stack
	conduit, err := net.Register(HotstuffEngine, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register hotstuff engine")
	}
	e.conduit = conduit

	return e, nil
}

// Identify uniquely identifies an event we know.
func (e *Engine) Identify(event interface{}) ([]byte, error) {
	return nil, errors.New("hotstuff identification not implemented")
}

// Retrieve processes a request for an event payload by hash.
func (e *Engine) Retrieve(hash []byte) (interface{}, error) {
	return nil, errors.New("hotstuff requesting not implemented")
}

// Receive processes event payloads received from the network stack.
func (e *Engine) Receive(node string, event interface{}) error {

	// handle the event
	var err error
	switch event.(type) {
	case *hotstuff.Block:
		err = errors.New("block handling not implemented")
	case *hotstuff.Vote:
		err = errors.New("vote handling not implemented")
	case *hotstuff.Timeout:
		err = errors.New("timeout handling not implemented")
	default:
		err = errors.Errorf("invalid hotstuff event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process hotstuff event")
	}

	return nil
}
