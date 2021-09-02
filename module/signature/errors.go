package signature

import (
	"errors"
)

var (
	ErrInvalidFormat      = errors.New("invalid signature format")
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")

	ErrInvalidInputs    = errors.New("invalid inputs")
	ErrDuplicatedSigner = errors.New("duplicated signer")
)
