package match

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

// reuse an error type
func ErrorExecutionResultExist(resultID flow.Identifier) error {
	return InvalidInput{
		Msg: fmt.Sprintf("execution result already exists in mempool, execution result ID: %v", resultID),
	}
}

func TestCustomizedError(t *testing.T) {
	var err error
	err = ErrorExecutionResultExist(flow.Identifier{0x11})

	var errType InvalidInput
	require.True(t, errors.As(err, &errType))

	err = fmt.Errorf("could not process, because: %w", err)
	require.True(t, errors.As(err, &errType))
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

	err = InvalidInput{
		Msg: fmt.Sprintf("invalid input: %v", err),
	}

	var errInvalidInput InvalidInput
	require.True(t, errors.As(err, &errInvalidInput))
	var errCustomizedError CustomizedError
	require.False(t, errors.As(err, &errCustomizedError))
	require.Equal(t, "invalid input: customized", err.Error())

	err = InvalidInput{
		Msg: "invalid input",
		Err: CustomizedError{
			Msg: "customized",
		},
	}
	require.True(t, errors.As(err, &errInvalidInput))
	require.True(t, errors.As(err, &errCustomizedError))
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
	require.True(t, errors.As(err, &errFieldsError))
}
