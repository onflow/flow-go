package storage

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var (
	// Note: there is another not found error: badger.ErrKeyNotFound. The difference between
	// badger.ErrKeyNotFound and storage.ErrNotFound is that:
	// badger.ErrKeyNotFound is the error returned by the badger API.
	// Modules in storage/badger and storage/badger/operation package both
	// return storage.ErrNotFound for not found error
	ErrNotFound = errors.New("key not found")

	ErrAlreadyExists = errors.New("key already exists")
	ErrDataMismatch  = errors.New("data for key is different")
)

type ResultAlreadyExistsErr struct {
	BlockID  flow.Identifier
	ResultID flow.Identifier
	err      error
}

func NewResultAlreadyExistsErrorf(msg string, blockID, resultID flow.Identifier, args ...interface{}) error {
	return ResultAlreadyExistsErr{
		BlockID:  blockID,
		ResultID: resultID,
		err:      fmt.Errorf(msg, args...),
	}
}

func (e ResultAlreadyExistsErr) Unwrap() error {
	return e.err
}

func (e ResultAlreadyExistsErr) Error() string {
	return e.err.Error()
}

func IsResultAlreadyExistsErr(err error) bool {
	var errResultAlreadyExistsErr ResultAlreadyExistsErr
	return errors.As(err, &errResultAlreadyExistsErr)
}
