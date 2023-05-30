package network

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/p2p"
)

// ErrHardThreshold indicates that the amount of RPC messages received exceeds hard threshold.
type ErrHardThreshold struct {
	// controlMsg the control message type.
	controlMsg p2p.ControlMessageType
	// amount the amount of control messages.
	amount uint64
	// hardThreshold configured hard threshold.
	hardThreshold uint64
}

func (e ErrHardThreshold) Error() string {
	return fmt.Sprintf("number of %s messges received exceeds the configured hard threshold: received %d hard threshold %d", e.controlMsg, e.amount, e.hardThreshold)
}

// NewHardThresholdErr returns a new ErrHardThreshold.
func NewHardThresholdErr(controlMsg p2p.ControlMessageType, amount, hardThreshold uint64) ErrHardThreshold {
	return ErrHardThreshold{controlMsg: controlMsg, amount: amount, hardThreshold: hardThreshold}
}

// IsErrHardThreshold returns true if an error is ErrHardThreshold
func IsErrHardThreshold(err error) bool {
	var e ErrHardThreshold
	return errors.As(err, &e)
}

// ErrInvalidLimitConfig indicates the validation limit is < 0.
type ErrInvalidLimitConfig struct {
	// controlMsg the control message type.
	controlMsg p2p.ControlMessageType
	err        error
}

func (e ErrInvalidLimitConfig) Error() string {
	return fmt.Errorf("invalid rpc control message %s validation limit configuration: %w", e.controlMsg, e.err).Error()
}

// NewInvalidLimitConfigErr returns a new ErrValidationLimit.
func NewInvalidLimitConfigErr(controlMsg p2p.ControlMessageType, err error) ErrInvalidLimitConfig {
	return ErrInvalidLimitConfig{controlMsg: controlMsg, err: err}
}

// IsErrInvalidLimitConfig returns whether an error is ErrInvalidLimitConfig.
func IsErrInvalidLimitConfig(err error) bool {
	var e ErrInvalidLimitConfig
	return errors.As(err, &e)
}
