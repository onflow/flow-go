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
