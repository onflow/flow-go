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
		require.False(
			t,
			collector.Collect(
				fmt.Errorf(
					"error wrapped: %w",
					NewOperationNotSupportedError("op1")),
			).CollectedFailure())

		require.True(t, collector.CollectedError())
		err := collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.False(t, IsFailure(err))

		nonFatal, fatal := SplitErrorTypes(err)
		require.Nil(t, fatal)
		require.NotNil(t, nonFatal)
		require.Equal(t, ErrCodeOperationNotSupportedError, nonFatal.Code())

		require.False(
			t,
			collector.Collect(
				NewInvalidArgumentErrorf("bad arg"),
			).CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error wrapped")
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "bad arg")
		require.False(t, IsFailure(err))

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
