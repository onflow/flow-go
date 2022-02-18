package io

import (
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminateOnFullDisk(t *testing.T) {
	// happy path
	t.Run("return nil on no error", func(t *testing.T) {
		result := TerminateOnFullDisk(nil)
		require.NoError(t, result)
	})
	t.Run("benign non disk related error should return the error", func(t *testing.T) {
		benignError := errors.New("benign error")
		result := TerminateOnFullDisk(benignError)
		require.ErrorIs(t, result, benignError)
	})
	// sad path
	t.Run("panic on full disk", func(t *testing.T) {
		// imitate badgerDB error wrapping
		badgerDiskFullError := syscall.ENOSPC
		defer func() {
			if rec := recover(); rec == nil {
				require.Fail(t, "code should panic")
			}
		}()
		err := TerminateOnFullDisk(badgerDiskFullError)
		require.NoError(t, err)
	})
}
