package network

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/p2plogging"
)

var (
	EmptyTargetList = errors.New("target list empty")
)

// ErrIllegalConnectionState indicates connection status to node is NotConnected but connections to node > 0
type ErrIllegalConnectionState struct {
	pid        peer.ID
	numOfConns int
}

func (e ErrIllegalConnectionState) Error() string {
	return fmt.Sprintf("unexpected connection status to peer %s: received NotConnected status while connection list is not empty %d ", p2plogging.PeerId(e.pid), e.numOfConns)
}

// NewConnectionStatusErr returns a new ErrIllegalConnectionState.
func NewConnectionStatusErr(pid peer.ID, numOfConns int) ErrIllegalConnectionState {
	return ErrIllegalConnectionState{pid: pid, numOfConns: numOfConns}
}

// IsErrConnectionStatus returns whether an error is ErrIllegalConnectionState
func IsErrConnectionStatus(err error) bool {
	var e ErrIllegalConnectionState
	return errors.As(err, &e)
}

// TransientError represents an error returned from a network layer function call
// which may be interpreted as non-critical. In general, we desire that all expected
// error return values are enumerated in a function's documentation - any undocumented
// errors are considered fatal. However, 3rd party libraries don't always conform to
// this standard, including the networking libraries we use. This error type can be
// used to wrap these 3rd party errors on the boundary into flow-go, to explicitly
// mark them as non-critical.
type TransientError struct {
	Err error
}

func (err TransientError) Error() string {
	return err.Err.Error()
}

func (err TransientError) Unwrap() error {
	return err.Err
}

func NewTransientErrorf(msg string, args ...interface{}) TransientError {
	return TransientError{
		Err: fmt.Errorf(msg, args...),
	}
}

func IsTransientError(err error) bool {
	var errTransient TransientError
	return errors.As(err, &errTransient)
}
