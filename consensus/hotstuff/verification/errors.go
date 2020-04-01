package verification

import (
	"errors"
)

var (
	ErrInvalidSigner = errors.New("invalid signer(s)")
	ErrInvalidFormat = errors.New("invalid signature format")
)
