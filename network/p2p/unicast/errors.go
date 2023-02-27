package unicast

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ErrDialInProgress indicates that the libp2p node is currently dialing the peer.
type ErrDialInProgress struct {
	pid peer.ID
}

func (e ErrDialInProgress) Error() string {
	return fmt.Sprintf("dialing to peer %s already in progress", e.pid.String())
}

// NewDialInProgressErr returns a new ErrDialInProgress.
func NewDialInProgressErr(pid peer.ID) ErrDialInProgress {
	return ErrDialInProgress{pid: pid}
}

// IsErrDialInProgress returns whether an error is ErrDialInProgress
func IsErrDialInProgress(err error) bool {
	var e ErrDialInProgress
	return errors.As(err, &e)
}

// ErrSecurityProtocolNegotiationFailed indicates security protocol negotiation failed during the stream factory connect attempt.
type ErrSecurityProtocolNegotiationFailed struct {
	pid peer.ID
	err error
}

func (e ErrSecurityProtocolNegotiationFailed) Error() string {
	return fmt.Errorf("failed to dial remote peer %s in stream factory invalid node ID: %w", e.pid.String(), e.err).Error()
}

// NewSecurityProtocolNegotiationErr returns a new ErrSecurityProtocolNegotiationFailed.
func NewSecurityProtocolNegotiationErr(pid peer.ID, err error) ErrSecurityProtocolNegotiationFailed {
	return ErrSecurityProtocolNegotiationFailed{pid: pid, err: err}
}

// IsErrSecurityProtocolNegotiationFailed returns whether an error is ErrSecurityProtocolNegotiationFailed.
func IsErrSecurityProtocolNegotiationFailed(err error) bool {
	var e ErrSecurityProtocolNegotiationFailed
	return errors.As(err, &e)
}
