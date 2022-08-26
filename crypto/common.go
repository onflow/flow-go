package crypto

import (
	"errors"
	"fmt"
)

//revive:disable:var-naming

// the `go generate` command requires bash scripting, `cmake` and `git`.
//go:generate bash ./build_dependency.sh

const (
	// Minimum targeted bits of security.
	// This is used as a reference but it doesn't mean all implemented primitives provide this minimum.
	securityBits = 128

	// Relic internal constant (related to exported constants above)
	// max byte length of bn_st set to 2048 bits
	maxScalarSize = 256

	// max relic PRG seed length in bytes
	maxRelicPrgSeed = 1 << 32
)

// invalidInputsError is an error returned when a crypto API receives invalid inputs.
// It allows a function caller differentiate unexpected program errors from errors caused by
// invalid inputs.
type invalidInputsError struct {
	error
}

func (e invalidInputsError) Unwrap() error {
	return e.error
}

// invalidInputsErrorf constructs a new invalidInputsError
func invalidInputsErrorf(msg string, args ...interface{}) error {
	return &invalidInputsError{
		error: fmt.Errorf(msg, args...),
	}
}

// IsInvalidInputsError checks if the input error is of a invalidInputsError type
func IsInvalidInputsError(err error) bool {
	var target *invalidInputsError
	return errors.As(err, &target)
}

var nilHasherError = errors.New("hasher cannot be nil")

// IsNilHasherError checks if the input error wraps a nilHasherError.
// nilHasherError is returned when a nil hasher is used.
func IsNilHasherError(err error) bool {
	return errors.Is(err, nilHasherError)
}

// invalidHasherSizeError is an error returned when a crypto API is called with a hasher
// with an output size not suited with the cryptographic operation.
type invalidHasherSizeError struct {
	error
}

func (e invalidHasherSizeError) Unwrap() error {
	return e.error
}

// invalidHasherSizeErrorf constructs a new invalidHasherSizeError
func invalidHasherSizeErrorf(msg string, args ...interface{}) error {
	return &invalidHasherSizeError{
		error: fmt.Errorf(msg, args...),
	}
}

// IsInvalidHasherSizeError checks if the input error is of a invalidHasherSizeError type.
// invalidHasherSizeError is an error returned when a crypto API is called with a hasher
// with an output size not suited with the cryptographic operation.
func IsInvalidHasherSizeError(err error) bool {
	var target *invalidHasherSizeError
	return errors.As(err, &target)
}
