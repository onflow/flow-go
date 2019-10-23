// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

// Engine represents an isolated process running across the peer-to-peer network
// as part of the node business logic. It provides the network layer with
// the necessary interface to process, identify and retrieve engine events.
type Engine interface {

	// Process will submit the given event to the engine for processing. It
	// returns an error so a node which has the given engine registered will not
	// propagate an event unless it was succcessfully processed by the engine.
	// The origin ID indicates the node which originally submitted the event to
	// the peer-to-peer network.
	Process(originID string, event interface{}) error

	// Identify allows a node to identify a given event uniquely while remaining
	// agnostic of event types. It will usually return the canonical hash of the
	// entity that the event is communicating about. In cases where no such hash
	// is available, identify can return an error and the network layer will use
	// a custom hash of the payload for caching.
	Identify(event interface{}) ([]byte, error)

	// Retrieve allows the network layer to retrieve an event from an engine by
	// its unique ID. It can only be used for events that can be uniquely
	// identified by the engine; events that used the internal network layer hash
	// can not (and do not need) to be retrieved from the engine.
	Retrieve(eventID []byte) (interface{}, error)
}
