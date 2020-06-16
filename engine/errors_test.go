package engine

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

// reuse an error type
func ErrorExecutionResultExist(resultID flow.Identifier) error {
	return NewInvalidInputErrorf("execution result already exists in mempool, execution result ID: %v", resultID)
}

func TestCustomizedError(t *testing.T) {
	var err error
	err = ErrorExecutionResultExist(flow.Identifier{0x11})

	require.True(t, IsInvalidInputError(err))

	// if an invalid error is wrapped with `fmt.Errorf`, it
	// is still an invalid error
	err = fmt.Errorf("could not process, because: %w", err)
	require.True(t, IsInvalidInputError(err))
}

type CustomizedError struct {
	Msg string
}

func (e CustomizedError) Error() string {
	return e.Msg
}

func TestErrorWrapping(t *testing.T) {
	var err error
	err = CustomizedError{
		Msg: "customized",
	}

	// wrap with invalid input
	err = NewInvalidInputErrorf("invalid input: %v", err)

	// when a customized error is wrapped with invalid input, it is
	// an invalid error
	require.True(t, IsInvalidInputError(err))
	require.Equal(t, "invalid input: customized", err.Error())

	// when a customized error is wrapped with invalid input, it is
	// no longer an customized error
	var errCustomizedError CustomizedError
	require.False(t, errors.As(err, &errCustomizedError))
}

var NoFieldError = errors.New("sentinal error")

type FieldsError struct {
	Field1 string
	Field2 int
	Err    error
}

func (e FieldsError) Error() string {
	return fmt.Sprintf("field1: %v, field2: %v, err: %v", e.Field1, e.Field2, e.Err)
}

func TestTypeCheck(t *testing.T) {
	var err error
	err = NoFieldError
	require.True(t, errors.Is(err, NoFieldError))

	err = FieldsError{
		Field1: "field1 missing",
		Field2: 100,
		Err:    NoFieldError,
	}
	var errFieldsError FieldsError
	// if an no field error is wrapped with fields error, then
	// it is a fields error
	require.True(t, errors.As(err, &errFieldsError))
}
