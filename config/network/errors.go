package network

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/p2p"
)

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
