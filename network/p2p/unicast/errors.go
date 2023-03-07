package unicast

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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

// ErrProtocolNotSupported indicates node is running on a different spork.
type ErrProtocolNotSupported struct {
	peerID      peer.ID
	protocolIDS []protocol.ID
	err         error
}

func (e ErrProtocolNotSupported) Error() string {
	return fmt.Errorf("failed to dial remote peer %s remote node is running on a different spork: %w, protocol attempted: %s", e.peerID.String(), e.err, e.protocolIDS).Error()
}

// NewProtocolNotSupportedErr returns a new ErrSecurityProtocolNegotiationFailed.
func NewProtocolNotSupportedErr(peerID peer.ID, protocolIDS []protocol.ID, err error) ErrProtocolNotSupported {
	return ErrProtocolNotSupported{peerID: peerID, protocolIDS: protocolIDS, err: err}
}

// IsErrProtocolNotSupported returns whether an error is ErrProtocolNotSupported.
func IsErrProtocolNotSupported(err error) bool {
	var e ErrProtocolNotSupported
	return errors.As(err, &e)
}

// ErrGaterDisallowedConnection wrapper around github.com/libp2p/go-libp2p/p2p/net/swarm.ErrGaterDisallowedConnection.
type ErrGaterDisallowedConnection struct {
	err error
}

func (e ErrGaterDisallowedConnection) Error() string {
	return fmt.Errorf("target node is not on the approved list of nodes: %w", e.err).Error()
}

// NewGaterDisallowedConnectionErr returns a new ErrGaterDisallowedConnection.
func NewGaterDisallowedConnectionErr(err error) ErrGaterDisallowedConnection {
	return ErrGaterDisallowedConnection{err: err}
}

// IsErrGaterDisallowedConnection returns whether an error is ErrGaterDisallowedConnection.
func IsErrGaterDisallowedConnection(err error) bool {
	var e ErrGaterDisallowedConnection
	return errors.As(err, &e)
}

// ErrMaxRetries  indicates retries completed with max retries without a successful attempt.
type ErrMaxRetries struct {
	attempts uint64
	err      error
}

func (e ErrMaxRetries) Error() string {
	return fmt.Errorf("retries failed max attempts reached %d: %w", e.attempts, e.err).Error()
}

// NewMaxRetriesErr returns a new ErrMaxRetries.
func NewMaxRetriesErr(attempts uint64, err error) ErrMaxRetries {
	return ErrMaxRetries{attempts: attempts, err: err}
}

// IsErrMaxRetries returns whether an error is ErrMaxRetries.
func IsErrMaxRetries(err error) bool {
	var e ErrMaxRetries
	return errors.As(err, &e)
}
