// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/pkg/codec"
	"github.com/dapperlabs/flow-go/pkg/model/flow"
	"github.com/dapperlabs/flow-go/pkg/naive"
)

const (
	FlowEngine = 1
)

// Engine represents the flow system engine, which runs the flow network with
// its different node roles.
type Engine struct {
	log     zerolog.Logger
	codec   codec.Codec
	conduit naive.Conduit
}

// NewEngine initializes a new flow system engine.
func NewEngine(log zerolog.Logger, codec codec.Codec, net naive.Network) (*Engine, error) {

	// initialize the flow engine
	e := &Engine{
		log:   log,
		codec: codec,
	}

	// register the flow engine with the network stack
	conduit, err := net.Register(FlowEngine, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register flow engine")
	}
	e.conduit = conduit

	return e, nil
}

// Identify uniquely identifies an event we know.
func (e *Engine) Identify(event interface{}) ([]byte, error) {
	return nil, errors.New("flow identification not implemented")
}

// Retrieve implements requesting of a payload from the flow.
func (e *Engine) Retrieve(hash []byte) (interface{}, error) {
	return nil, errors.New("flow requesting not implemented")
}

// Receive implements reception of an event.
func (e *Engine) Receive(node string, event interface{}) error {

	// process the entity
	var err error
	switch event.(type) {
	case *flow.Collection:
		err = errors.New("collection handling not implemented")
	case *flow.Receipt:
		err = errors.New("receipt handling not implemented")
	case *flow.Approval:
		err = errors.New("approval handling not implemented")
	case *flow.Seal:
		err = errors.New("seal handling not implemented")
	default:
		err = errors.Errorf("invalid flow event type (%T)", e)
	}
	if err != nil {
		return errors.Wrap(err, "could not process flow event")
	}

	return nil
}
