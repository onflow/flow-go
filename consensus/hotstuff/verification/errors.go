package verification

import (
	"errors"
)

var (
	ErrInvalidSignature   = errors.New("invalid signer(s)")
	ErrInvalidFormat      = errors.New("invalid signature format")
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")
)
