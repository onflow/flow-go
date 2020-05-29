package match

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

func ErrorExecutionResultExist(resultID flow.Identifier) error {
	return InvalidInput{
		Msg: fmt.Sprintf("execution result already exists in mempool, execution result ID: %v", resultID),
	}
}

func TestCustomizedError(t *testing.T) {
	var err error
	err = ErrorExecutionResultExist(flow.Identifier{0x11})

	require.True(t, errors.Is(err, InvalidInput{}))

	err = fmt.Errorf("could not process, because: %w", err)
	require.True(t, errors.Is(err, InvalidInput{}))
}

type CustomizedError struct {
	Msg string
}

func (e CustomizedError) Error() string {
	return e.Msg
}

func (e CustomizedError) Is(other error) bool {
	_, ok := other.(CustomizedError)
	return ok
}

func TestErrorWrapping(t *testing.T) {
	var err error
	err = CustomizedError{
		Msg: "customized",
	}

	err = InvalidInput{
		Msg: fmt.Sprintf("invalid input: %v", err),
	}
	require.True(t, errors.Is(err, InvalidInput{}))
	// do this think we would want to assert a wrapped error
	require.False(t, errors.Is(err, CustomizedError{}))
	fmt.Printf("%v\n", err.Error())

	err = InvalidInput{
		Msg: "invalid input",
		Err: CustomizedError{
			Msg: "customized",
		},
	}
	require.True(t, errors.Is(err, InvalidInput{}))
	require.True(t, errors.Is(err, CustomizedError{}))
}
