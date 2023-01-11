package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorsCollector(t *testing.T) {
	t.Run("basic collection", func(t *testing.T) {
		collector := ErrorsCollector{}

		// Collecting nil is ok
		require.False(t, collector.Collect(nil).CollectedFailure())
		require.False(t, collector.CollectedError())
		require.Nil(t, collector.ErrorOrNil())

		// Collected non-fatal errors
		err1 := NewOperationNotSupportedError("op1")
		require.False(
			t,
			collector.Collect(fmt.Errorf("error wrapped: %w", err1)).
				CollectedFailure())

		require.True(t, collector.CollectedError())
		err := collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.False(t, IsFailure(err))

		require.True(t, IsOperationNotSupportedError(err))
		found := Find(err, ErrCodeOperationNotSupportedError)
		require.Equal(t, err1, found)

		nonFatal, fatal := SplitErrorTypes(err)
		require.Nil(t, fatal)
		require.NotNil(t, nonFatal)
		require.Equal(t, ErrCodeOperationNotSupportedError, nonFatal.Code())

		err2 := NewInvalidArgumentErrorf("bad arg")
		require.False(t, collector.Collect(err2).CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error wrapped")
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "bad arg")
		require.False(t, IsFailure(err))

		require.True(t, IsOperationNotSupportedError(err))
		found = Find(err, ErrCodeOperationNotSupportedError)
		require.Equal(t, err1, found)

		require.True(t, IsInvalidArgumentError(err))
		found = Find(err, ErrCodeInvalidArgumentError)
		require.Equal(t, err2, found)

		nonFatal, fatal = SplitErrorTypes(err)
		require.Nil(t, fatal)
		require.NotNil(t, nonFatal)
		require.Equal(t, ErrCodeOperationNotSupportedError, nonFatal.Code())

		// Collected fatal error
		require.True(
			t,
			collector.Collect(
				fmt.Errorf(
					"failure wrapped: %w",
					NewLedgerFailure(fmt.Errorf("fatal1"))),
			).CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error wrapped")
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "bad arg")
		require.ErrorContains(t, err, "failure wrapped")
		require.ErrorContains(t, err, "fatal1")
		require.True(t, IsFailure(err))

		// Note: when a fatal error is collected, the non-fatal errors are no
		// longer accessible.
		require.False(t, IsOperationNotSupportedError(err))
		require.False(t, IsInvalidArgumentError(err))
		require.True(t, IsLedgerFailure(err))

		nonFatal, fatal = SplitErrorTypes(err)
		require.Nil(t, nonFatal)
		require.NotNil(t, fatal)
		require.Equal(t, FailureCodeLedgerFailure, fatal.Code())

		// Collecting a non-fatal error after a fatal error should still be
		// fatal
		require.True(
			t,
			collector.Collect(
				NewOperationNotSupportedError("op3"),
			).CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error wrapped")
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "bad arg")
		require.ErrorContains(t, err, "op3")
		require.ErrorContains(t, err, "failure wrapped")
		require.ErrorContains(t, err, "fatal1")
		require.True(t, IsFailure(err))

		nonFatal, fatal = SplitErrorTypes(err)
		require.Nil(t, nonFatal)
		require.NotNil(t, fatal)
		require.Equal(t, FailureCodeLedgerFailure, fatal.Code())

		// Collected multiple fatal errors (This should never happen, but just
		// in case)
		require.True(
			t,
			collector.Collect(fmt.Errorf("fatal2")).CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error wrapped")
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "bad arg")
		require.ErrorContains(t, err, "op3")
		require.ErrorContains(t, err, "failure wrapped")
		require.ErrorContains(t, err, "fatal1")
		require.ErrorContains(t, err, "fatal2")
		require.True(t, IsFailure(err))

		nonFatal, fatal = SplitErrorTypes(err)
		require.Nil(t, nonFatal)
		require.NotNil(t, fatal)
		require.Equal(t, FailureCodeLedgerFailure, fatal.Code())
	})

	t.Run("failure only", func(t *testing.T) {
		collector := ErrorsCollector{}

		require.True(
			t,
			collector.Collect(fmt.Errorf("fatal1")).CollectedFailure())

		require.True(t, collector.CollectedError())
		err := collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "fatal1")
		require.True(t, IsFailure(err))

		nonFatal, fatal := SplitErrorTypes(err)
		require.Nil(t, nonFatal)
		require.NotNil(t, fatal)
		require.Equal(t, FailureCodeUnknownFailure, fatal.Code())
	})
}
