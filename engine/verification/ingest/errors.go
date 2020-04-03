package ingest

import (
	"errors"
)

var (
	ErrInvType = errors.New("could not process message: invalid type")
)
