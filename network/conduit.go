// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Conduit represents the interface for engines to communicate over the
// peer-to-peer network. Upon registration with the network, each engine is
// assigned a conduit, which it can use to communicate across the network in
// a network-agnostic way. In the background, the network layer connects all
// engines with the same ID over a shared bus, accessible through the conduit.
type Conduit interface {

	// Transmit submits a message to the network layer for reliable delivery
	// to the specified recipients. If the message can not be delivered to any
	// one of the recipients, an error will be returned.
	Transmit(message interface{}, recipientIDs ...flow.Identifier) error

	// Send submits a message to the network layer for reliable delivery to the
	// specified number of recipients. The specified selector will determine
	// the subset of nodes that serves as the pool from which the recipients
	// will be randomly drawn. If an insufficient number of recipients is
	// available within the subset, the function will return an error.
	Send(message interface{}, num uint, selector flow.IdentityFilter) error

	// Publish submits a message to the network layer for unreliable delivery
	// to subscribers of the given message on the network layer. It uses a
	// publish-subscribe layer and can thus not guarantee that the specified
	// recipients received the message.
	Publish(message interface{}, restrictor flow.IdentityFilter) error
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
