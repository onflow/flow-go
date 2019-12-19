package storage

import "errors"

var (
	NotFoundErr         = errors.New("not found")
	DifferentDataErr    = errors.New("different data for same key")
	KeyAlreadyExistsErr = errors.New("key already exists")
)
