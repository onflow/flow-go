package p2pconfig

import (
	"errors"
	"fmt"

	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// InvalidLimitConfigError indicates the validation limit is < 0.
type InvalidLimitConfigError struct {
	err error
}

func (e InvalidLimitConfigError) Error() string {
	return e.err.Error()
}

func (e InvalidLimitConfigError) Unwrap() error {
	return e.err
}

// NewInvalidLimitConfigErr returns a new ErrValidationLimit.
func NewInvalidLimitConfigErr(controlMsg p2pmsg.ControlMessageType, err error) InvalidLimitConfigError {
	return InvalidLimitConfigError{fmt.Errorf("invalid rpc control message %s validation limit configuration: %w", controlMsg, err)}
}

// IsInvalidLimitConfigError returns whether an error is ErrInvalidLimitConfig.
func IsInvalidLimitConfigError(err error) bool {
	var e InvalidLimitConfigError
	return errors.As(err, &e)
}
