package signature

import (
	"errors"
)

var (
	ErrInvalidFormat      = errors.New("invalid signature")
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")
	ErrDuplicatedSigner   = errors.New("duplicated signer")
)
