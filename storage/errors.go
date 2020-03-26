package storage

import "errors"

var (
	ErrNotFound       = errors.New("key not found")
	ErrAlreadyExists  = errors.New("key already exists")
	ErrDataMismatch   = errors.New("data for key is different")
	ErrAlreadyIndexed = errors.New("entity already indexed")
)
