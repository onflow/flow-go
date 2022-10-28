package model

import (
	"errors"
	"fmt"
)

// StructureInvalidError is returned by a StructureValidator if the
type StructureInvalidError struct {
	err error
}

func NewStructureInvalidError(msg string, args ...interface{}) error {
	return StructureInvalidError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e StructureInvalidError) Unwrap() error {
	return e.err
}

func (e StructureInvalidError) Error() string {
	return e.err.Error()
}

func IsStructureInvalidError(err error) bool {
	var structureInvalidErr StructureInvalidError
	return errors.As(err, &structureInvalidErr)
}

// StructureValidator is a model type which exposes a method to validate basic structural
// validity of a message or model. This DOES NOT enforce business logic rules,
// it only enforces that required fields are non-nil and no fields are malformed.
type StructureValidator interface {
	// StructureValid validates the model's basic structural validity: all required fields
	// are non-nil and well-formed. It does NOT enforce business logic rules.
	// Implementations must be non-blocking and computationally inexpensive, because
	// structural validity is enforced early in the message processing flow.
	// Returns StructureInvalidError if the model is structurally invalid.
	StructureValid() error
}
