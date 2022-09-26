package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestErrorHandling(t *testing.T) {

	t.Run("test nonfatal error detection", func(t *testing.T) {
		e1 := NewOperationNotSupportedError("some operations")
		e2 := fmt.Errorf("some other errors: %w", e1)
		e3 := NewInvalidProposalSignatureError(flow.EmptyAddress, 0, e2)

		txErr, vmErr := SplitErrorTypes(e3)
		require.Nil(t, vmErr)
		require.Equal(t, e3, txErr)
	})

	t.Run("test fatal error detection", func(t *testing.T) {
		e1 := NewOperationNotSupportedError("some operations")
		e2 := NewLedgerFailure(e1)
		e3 := fmt.Errorf("some other errors: %w", e2)
		e4 := NewInvalidProposalSignatureError(flow.EmptyAddress, 0, e3)

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
