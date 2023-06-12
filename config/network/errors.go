package network

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/p2p"
)

// ErrInvalidLimitConfig indicates the validation limit is < 0.
type ErrInvalidLimitConfig struct {
	err error
}

func (e ErrInvalidLimitConfig) Error() string {
	return e.err.Error()
}

func (e ErrInvalidLimitConfig) Unwrap() error {
	return e.err
}

// NewInvalidLimitConfigErr returns a new ErrValidationLimit.
func NewInvalidLimitConfigErr(controlMsg p2p.ControlMessageType, err error) ErrInvalidLimitConfig {
	return ErrInvalidLimitConfig{fmt.Errorf("invalid rpc control message %s validation limit configuration: %w", controlMsg, err)}
}

// IsErrInvalidLimitConfig returns whether an error is ErrInvalidLimitConfig.
func IsErrInvalidLimitConfig(err error) bool {
	var e ErrInvalidLimitConfig
	return errors.As(err, &e)
}
