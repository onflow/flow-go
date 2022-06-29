package network

import (
	"errors"
	"fmt"
)

var (
	EmptyTargetList = errors.New("target list empty")
)

// BenignNetworkingError covers the entire class of benign networking errors.
// This error is only returned, if the networking layer is still fully functional
// despite the encountering the error condition.
type BenignNetworkingError struct {
	err error
}

func NewBenignNetworkingErrorf(msg string, args ...interface{}) error {
	return BenignNetworkingError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e BenignNetworkingError) Error() string { return e.err.Error() }
func (e BenignNetworkingError) Unwrap() error { return e.err }

// IsBenignNetworkingError returns whether err is an BenignNetworkingError
func IsBenignNetworkingError(err error) bool {
	var e BenignNetworkingError
	return errors.As(err, &e)
}
