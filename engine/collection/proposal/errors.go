package proposal

import (
	"errors"
)

var (
	// ErrEmptyTxpool indicates that no collection can be formed because the
	// transaction pool is empty.
	ErrEmptyTxpool = errors.New("transaction pool empty")
)
