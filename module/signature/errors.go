package signature

import (
	"errors"
)

var (
	ErrInvalidFormat      = errors.New("invalid signature format")
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")
	ErrInvalidSignerIdx   = errors.New("invalid signer index")
	ErrDuplicatedSigner   = errors.New("duplicated signer")
)
