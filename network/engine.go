// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package network

import (
	"github.com/onflow/flow-go/model/flow"
)

// Engine represents an isolated process running across the peer-to-peer network
// as part of the node business logic. It provides the network layer with
// the necessary interface to forward events to engines for processing.
// TODO: DEPRECATED replace with MessageProcessor
type Engine interface {

	// SubmitLocal submits an event originating on the local node.
	SubmitLocal(event interface{})

	// Submit submits the given event from the node with the given origin ID
	// for processing in a non-blocking manner. It returns instantly and logs
	// a potential processing error internally when done.
	Submit(channel Channel, originID flow.Identifier, event interface{})

	// ProcessLocal processes an event originating on the local node.
	ProcessLocal(event interface{}) error

	// Process processes the given event from the node with the given origin ID
	// in a blocking manner. It returns the potential processing error when
	// done.
	Process(channel Channel, originID flow.Identifier, event interface{}) error
}

type MessageProcessor interface {
	Process(channel Channel, originID flow.Identifier, message interface{}) error
}
