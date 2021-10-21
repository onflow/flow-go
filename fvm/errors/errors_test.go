package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorHandling(t *testing.T) {

	t.Run("test nonfatal error detection", func(t *testing.T) {
		e1 := &OperationNotSupportedError{"some operations"}
		e2 := fmt.Errorf("some other errors: %w", e1)
		e3 := &InvalidProposalSignatureError{err: e2}

		txErr, vmErr := SplitErrorTypes(e3)
		require.Nil(t, vmErr)
		require.Equal(t, e3, txErr)
	})

	t.Run("test fatal error detection", func(t *testing.T) {
		e1 := &OperationNotSupportedError{"some operations"}
		e2 := &LedgerFailure{e1}
		e3 := fmt.Errorf("some other errors: %w", e2)
		e4 := &InvalidProposalSignatureError{err: e3}

		txErr, vmErr := SplitErrorTypes(e4)
		require.Nil(t, txErr)
		require.Equal(t, e2, vmErr)
	})

	t.Run("unknown error", func(t *testing.T) {
		e1 := fmt.Errorf("some unknown errors")
		txErr, vmErr := SplitErrorTypes(e1)
		require.Nil(t, txErr)
		require.NotNil(t, vmErr)
	})
}
