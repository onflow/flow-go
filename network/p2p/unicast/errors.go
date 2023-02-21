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
