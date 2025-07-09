package network

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/channels"
)

// Engine represents an isolated process running across the peer-to-peer network
// as part of the node business logic. It provides the network layer with
// the necessary interface to forward events to engines for processing.
// Deprecated: Use MessageProcessor instead
type Engine interface {
	module.ReadyDoneAware

	// SubmitLocal submits an event originating on the local node.
	// Deprecated: To asynchronously communicate a local message between components:
	// * Define a message queue on the component receiving the message
	// * Define a function (with a concrete argument type) on the component receiving
	//   the message, which adds the message to the message queue
	SubmitLocal(event interface{})

	// Submit submits the given event from the node with the given origin ID
	// for processing in a non-blocking manner. It returns instantly and logs
	// a potential processing error internally when done.
	// Deprecated: Only applicable for use by the networking layer, which should use MessageProcessor instead
	Submit(channel channels.Channel, originID flow.Identifier, event interface{})

	// ProcessLocal processes an event originating on the local node.
	// Deprecated: To synchronously process a local message:
	// * Define a function (with a concrete argument type) on the component receiving
	//   the message, which blocks until the message is processed
	ProcessLocal(event interface{}) error

	// Process processes the given event from the node with the given origin ID
	// in a blocking manner. It returns the potential processing error when
	// done.
	// Deprecated: Only applicable for use by the networking layer, which should use MessageProcessor instead
	Process(channel channels.Channel, originID flow.Identifier, event interface{}) error
}

// MessageProcessor represents a component which receives messages from the
// networking layer. Since these messages come from other nodes, which may
// be Byzantine, implementations must expect and handle arbitrary message inputs
// (including invalid message types, malformed messages, etc.). Because of this,
// node-internal messages should NEVER be submitted to a component using Process.
type MessageProcessor interface {
	// Process is exposed by engines to accept messages from the networking layer.
	// Implementations of Process should be non-blocking. In general, Process should
	// only queue the message internally by the engine for later async processing.
	//
	// TODO: This function should not return an error.
	//  The networking layer's responsibility is fulfilled once it delivers a message to an engine.
	//  It does not possess the context required to handle errors that may arise during an engine's processing
	//  of the message, as error handling for message processing falls outside the domain of the networking layer.
	//  Consequently, it is reasonable to remove the error from the Process function's signature,
	//  since returning an error to the networking layer would not be useful in this context.
	Process(channel channels.Channel, originID flow.Identifier, message interface{}) error
}
