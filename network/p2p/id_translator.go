package p2p

import (
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

var (
	// ErrInvalidId indicates that the given ID (either `peer.ID` or `flow.Identifier`) has an invalid format.
	ErrInvalidId = errors.New("empty peer ID")

	// ErrUnknownId indicates that the given ID (either `peer.ID` or `flow.Identifier`) is unknown.
	ErrUnknownId = errors.New("unknown ID")
)

// IDTranslator provides an interface for converting from Flow ID's to LibP2P peer ID's
// and vice versa.
type IDTranslator interface {
	// GetPeerID returns the peer ID for the given `flow.Identifier`.
	// During normal operations, the following error returns are expected
	//  * ErrUnknownId if the given Identifier is unknown
	//  * ErrInvalidId if the given Identifier has an invalid format.
	//    This error return is only possible, when the Identifier is generated from some
	//    other input (e.g. the node's public key). While the Flow protocol itself makes
	//    no assumptions about the structure of the `Identifier`, protocol extensions
	//    (e.g. for Observers) might impose a strict relationship between both flow
	//    Identifier and peer ID, in which case they can use this error.
	// TODO: implementations do not fully adhere to this convention on error returns
	GetPeerID(flow.Identifier) (peer.ID, error)

	// GetFlowID returns the `flow.Identifier` for the given `peer.ID`.
	// During normal operations, the following error returns are expected
	//  * ErrUnknownId if the given Identifier is unknown
	//  * ErrInvalidId if the given Identifier has an invalid format.
	//    This error return is only possible, when the Identifier is generated from some
	//    other input (e.g. the node's public key). While the Flow protocol itself makes
	//    no assumptions about the structure of the `Identifier`, protocol extensions
	//    (e.g. for Observers) might impose a strict relationship between both flow
	//    Identifier and peer ID, in which case they can use this error.
	// TODO: implementations do not fully adhere to this convention on error returns
	GetFlowID(peer.ID) (flow.Identifier, error)
}
