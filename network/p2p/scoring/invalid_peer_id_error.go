package scoring

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

type peerIdStatus string

const (
	// PeerIdStatusUnknown indicates that the peer id is unknown.
	PeerIdStatusUnknown peerIdStatus = "unknown identity"
	// PeerIdStatusEjected indicates that the peer id belongs to an identity that has been ejected.
	PeerIdStatusEjected peerIdStatus = "ejected identity"
)

// InvalidPeerIDError indicates that a peer has an invalid peer id, i.e., it is not held by an authorized Flow identity.
type InvalidPeerIDError struct {
	peerId peer.ID      // the peer id that is not hold by any authorized identity.
	status peerIdStatus // the status of the peer id.
}

func NewInvalidPeerIDError(peerId peer.ID, status peerIdStatus) error {
	return InvalidPeerIDError{
		peerId: peerId,
		status: status,
	}
}

func (e InvalidPeerIDError) Error() string {
	return fmt.Sprintf("invalid peer id: %s reason: %s", e.peerId, e.status)
}

func IsInvalidPeerIDError(this error) bool {
	var err InvalidPeerIDError
	return errors.As(this, &err)
}
