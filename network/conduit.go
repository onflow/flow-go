// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type ConduitFactory interface {
	NewConduit(context.Context, context.CancelFunc, Channel) (Conduit, error)
	RegisterAdapter(Adapter) error
}

// Conduit represents the interface for engines to communicate over the
// peer-to-peer network. Upon registration with the network, each engine is
// assigned a conduit, which it can use to communicate across the network in
// a network-agnostic way. In the background, the network layer connects all
// engines with the same ID over a shared bus, accessible through the conduit.
type Conduit interface {

	// Publish submits an event to the network layer for unreliable delivery
	// to subscribers of the given event on the network layer. It uses a
	// publish-subscribe layer and can thus not guarantee that the specified
	// recipients received the event.
	// The event is published on the channels of this Conduit and will be received
	// by the nodes specified as part of the targetIDs
	Publish(event interface{}, targetIDs ...flow.Identifier) error

	// Unicast sends the event in a reliable way to the given recipient.
	// It uses 1-1 direct messaging over the underlying network to deliver the event.
	// It returns an error if the unicast fails.
	Unicast(event interface{}, targetID flow.Identifier) error

	// Multicast unreliably sends the specified event over the channel
	// to the specified number of recipients selected from the specified subset.
	// The recipients are selected randomly from the targetIDs.
	Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error

	// Close unsubscribes from the channels of this conduit. After calling close,
	// the conduit can no longer be used to send a message.
	Close() error
}

// PeerUnreachableError is the error when submitting events to target fails due to the
// target peer is unreachable
type PeerUnreachableError struct {
	Err error
}

// NewPeerUnreachableError creates a PeerUnreachableError instance with an error
func NewPeerUnreachableError(err error) error {
	return PeerUnreachableError{
		Err: err,
	}
}

// Unwrap returns the wrapped error value
func (e PeerUnreachableError) Unwrap() error {
	return e.Err
}

func (e PeerUnreachableError) Error() string {
	return fmt.Sprintf("%v", e.Err)
}

// IsPeerUnreachableError returns whether the given error is PeerUnreachableError
func IsPeerUnreachableError(e error) bool {
	var err PeerUnreachableError
	return errors.As(e, &err)
}

// AllPeerUnreachableError returns whether all errors are PeerUnreachableError
func AllPeerUnreachableError(errs ...error) bool {
	for _, err := range errs {
		if !IsPeerUnreachableError(err) {
			return false
		}
	}
	return true
}
