package io

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestMoveFile(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		src := filepath.Join(dir, "source.file")
		dst := filepath.Join(dir, "destination.file")
		sampleBytes := []byte{2, 1, 3, 7}

		require.NoError(t, os.WriteFile(src, sampleBytes, 0644))
		require.NoFileExists(t, dst)

		require.NoError(t, MoveFile(src, dst))

		require.NoFileExists(t, src)
		require.FileExists(t, dst)

		readBytes, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, sampleBytes, readBytes)
	})
}

func TestMoveFile_FallbackToCopyOnCrossDeviceError(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		src := filepath.Join(dir, "source.file")
		dst := filepath.Join(dir, "destination.file")
		sampleBytes := []byte{2, 1, 3, 7}

		require.NoError(t, os.WriteFile(src, sampleBytes, 0644))

		renameCalled := false
		renameFn := func(_, _ string) error {
			renameCalled = true
			return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.EXDEV}
		}

		require.NoError(t, moveFile(src, dst, renameFn))
		require.True(t, renameCalled)

		require.NoFileExists(t, src)
		require.FileExists(t, dst)

		readBytes, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, sampleBytes, readBytes)
	})
}

func TestMoveFile_ReturnsErrorOnNonCrossDeviceFailure(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		src := filepath.Join(dir, "source.file")
		dst := filepath.Join(dir, "destination.file")

		require.NoError(t, os.WriteFile(src, []byte{1, 2, 3}, 0644))

		renameFn := func(_, _ string) error {
			return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.ENOENT}
		}

		err := moveFile(src, dst, renameFn)
		require.Error(t, err)
		require.FileExists(t, src)
		require.NoFileExists(t, dst)
	})
}

func TestIsCrossDeviceRenameError(t *testing.T) {
	t.Run("detects EXDEV link error", func(t *testing.T) {
		err := &os.LinkError{Op: "rename", Old: "/a", New: "/b", Err: syscall.EXDEV}
		require.True(t, isCrossDeviceRenameError(err))
	})

	t.Run("detects generic cross-device message", func(t *testing.T) {
		err := &os.LinkError{Op: "rename", Old: "/a", New: "/b", Err: errors.New("invalid cross-device link")}
		require.True(t, isCrossDeviceRenameError(err))
	})

	t.Run("non-link error returns false", func(t *testing.T) {
		require.False(t, isCrossDeviceRenameError(errors.New("some error")))
	})

	t.Run("non-cross-device link error returns false", func(t *testing.T) {
		err := &os.LinkError{Op: "rename", Old: "/a", New: "/b", Err: syscall.ENOENT}
		require.False(t, isCrossDeviceRenameError(err))
	})
}

func TestCopyFileAndRemoveSource(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		src := filepath.Join(dir, "source.file")
		dst := filepath.Join(dir, "destination.file")
		sampleBytes := []byte{2, 1, 3, 7}

		require.NoError(t, os.WriteFile(src, sampleBytes, 0644))
		require.NoFileExists(t, dst)

		require.NoError(t, copyFileAndRemoveSource(src, dst))

		require.NoFileExists(t, src)
		require.FileExists(t, dst)

		readBytes, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, sampleBytes, readBytes)
	})
}
