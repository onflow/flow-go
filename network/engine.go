// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Engine represents an isolated process running across the peer-to-peer network
// as part of the node business logic. It provides the network layer with
// the necessary interface to forward events to engines for processing.
type Engine interface {

	// Process will submit the given event to the engine for processing. It
	// returns an error so a node which has the given engine registered will not
	// propagate an event unless it was successfully processed by the engine.
	// The origin ID indicates the node which originally submitted the event to
	// the peer-to-peer network.
	Process(originID flow.Identifier, event interface{}) error
}
