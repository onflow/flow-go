package errors

import (
	"fmt"
	"github.com/rs/zerolog"
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

func TestFirstOrFailure(t *testing.T) {
	e1 := fmt.Errorf("some error 1")
	e2 := fmt.Errorf("some error 2")
	f1 := &UnknownFailure{e1}
	f2 := &UnknownFailure{e1}
	l := zerolog.Nop()

	t.Run("nil, nil -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, nil, nil)
		require.Equal(t, nil, err)
	})
	t.Run("e, nil -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, e1, nil)
		require.Equal(t, e1, err)
	})

	t.Run("nil, e -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, nil, e2)
		require.Equal(t, e2, err)
	})

	t.Run("e, e -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, e1, e2)
		require.Equal(t, e1, err)
	})
	t.Run("f, e -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, f1, e2)
		require.Equal(t, f1, err)
	})
	t.Run("f, f -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, f1, f2)
		require.Equal(t, f1, err)
	})
	t.Run("e, f -> nil", func(t *testing.T) {
		err := FirstOrFailure(l, e1, f2)
		require.Equal(t, f2, err)
	})
}
