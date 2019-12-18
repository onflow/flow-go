package storage

import "fmt"

type Error interface {
	isNotFound() bool
	Cause() error
	Error() string
}

type BasicStorageError struct {
	err error
	msg string
}

func (err BasicStorageError) isNotFound() bool {
	return false
}

func (err BasicStorageError) Cause() error {
	return err.err
}

func (err BasicStorageError) Error() string {
	if err.err != nil {
		return err.msg + ": " + err.err.Error()
	}
	return err.msg
}

func WrapStorageError(err error, msg string) Error {
	return BasicStorageError{err: err, msg: msg}
}

func WrapStorageErrorf(err error, msg string, keys ...interface{}) Error {
	return BasicStorageError{err: err, msg: fmt.Sprintf(msg, keys...)}
}

func NewStorageError(msg string, args ...interface{}) Error {
	return BasicStorageError{err: fmt.Errorf(msg, args...)}
}

// Error indicating that requested data was not found
// and no other errors occurred during querying
type NotFoundError struct {
	BasicStorageError
	key []byte
}

func (err NotFoundError) Error() string {
	return fmt.Sprintf("data under key n%v not found", err.key)
}

func (err NotFoundError) isNotFound() bool {
	return true
}

func NewNotFoundError(key []byte) Error {
	return NotFoundError{key: key}
}

type DifferentDataUnderKey struct {
	BasicStorageError
	key []byte
}

func (err DifferentDataUnderKey) Error() string {
	return fmt.Sprintf("data under key n%v exists and its different to data requested to be inserted", err.key)
}

func NewDifferentDataUnderKey(key []byte) error {
	return DifferentDataUnderKey{key: key}
}
