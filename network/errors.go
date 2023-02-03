package network

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	EmptyTargetList = errors.New("target list empty")
)

// ErrUnexpectedConnectionStatus indicates connection status to node is NotConnected but connections to node > 0
type ErrUnexpectedConnectionStatus struct {
	pid        peer.ID
	numOfConns int
}

func (e ErrUnexpectedConnectionStatus) Error() string {
	return fmt.Sprintf("unexpected connection status to peer %s: received NotConnected status while connection list is not empty %d ", e.pid.String(), e.numOfConns)
}

// NewConnectionStatusErr returns a new ErrUnexpectedConnectionStatus.
func NewConnectionStatusErr(pid peer.ID, numOfConns int) ErrUnexpectedConnectionStatus {
	return ErrUnexpectedConnectionStatus{pid: pid, numOfConns: numOfConns}
}

// IsErrConnectionStatus returns whether an error is ErrUnexpectedConnectionStatus
func IsErrConnectionStatus(err error) bool {
	var e ErrUnexpectedConnectionStatus
	return errors.As(err, &e)
}
