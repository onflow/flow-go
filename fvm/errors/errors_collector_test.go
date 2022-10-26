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
			collector.Collect(NewOperationNotSupportedError("op1")).
				CollectedFailure())

		require.True(t, collector.CollectedError())
		err := collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.False(t, IsFailure(err))

		require.False(
			t,
			collector.Collect(NewOperationNotSupportedError("op2")).
				CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "op2")
		require.False(t, IsFailure(err))

		// Collected fatal error
		require.True(
			t,
			collector.Collect(fmt.Errorf("fatal1")).CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "op2")
		require.ErrorContains(t, err, "fatal1")
		require.True(t, IsFailure(err))

		// Collecting a non-fatal error after a fatal error should still be
		// fatal
		require.True(
			t,
			collector.Collect(NewOperationNotSupportedError("op3")).
				CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "op2")
		require.ErrorContains(t, err, "op3")
		require.ErrorContains(t, err, "fatal1")
		require.True(t, IsFailure(err))

		// Collected multiple fatal errors (This should never happen, but just
		// in case)
		require.True(t, collector.Collect(fmt.Errorf("fatal2")).
			CollectedFailure())

		require.True(t, collector.CollectedError())
		err = collector.ErrorOrNil()
		require.NotNil(t, err)
		require.ErrorContains(t, err, "op1")
		require.ErrorContains(t, err, "op2")
		require.ErrorContains(t, err, "op3")
		require.ErrorContains(t, err, "fatal1")
		require.ErrorContains(t, err, "fatal2")
		require.True(t, IsFailure(err))
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

	})
}
