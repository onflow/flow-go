package common_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/utils/unittest"
)

// createFakeCheckpoint creates empty stub files for all 18 V6 checkpoint files
// (1 header + 16 subtrie + 1 top-trie) in dir using the given base name.
func createFakeCheckpoint(t *testing.T, dir, name string) {
	t.Helper()
	for _, p := range wal.CheckpointV6AllFilePaths(dir, name) {
		f, err := os.Create(p)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}
}

// TestMoveCheckpointFiles_Success verifies that all 18 checkpoint files are moved to
// the destination directory with the new base name, and that no source files remain.
func TestMoveCheckpointFiles_Success(t *testing.T) {
	unittest.RunWithTempDir(t, func(base string) {
		srcDir := filepath.Join(base, "src")
		dstDir := filepath.Join(base, "dst")
		require.NoError(t, os.MkdirAll(srcDir, 0755))

		srcName := "checkpoint.00000005"
		dstName := "checkpoint.00000007"

		createFakeCheckpoint(t, srcDir, srcName)

		require.NoError(t, common.MoveCheckpointFiles(srcDir, srcName, dstDir, dstName))

		srcPaths := wal.CheckpointV6AllFilePaths(srcDir, srcName)
		dstPaths := wal.CheckpointV6AllFilePaths(dstDir, dstName)

		require.Len(t, dstPaths, 18, "expected 18 destination files")

		for i, dst := range dstPaths {
			_, err := os.Stat(dst)
			require.NoError(t, err, "expected destination file %d to exist: %s", i, dst)
		}

		for i, src := range srcPaths {
			_, err := os.Stat(src)
			require.True(t, os.IsNotExist(err), "expected source file %d to be gone: %s", i, src)
		}
	})
}

// TestMoveCheckpointFiles_MissingSourceFile verifies that the function returns an error
// without moving any files when at least one source checkpoint file is absent.
func TestMoveCheckpointFiles_MissingSourceFile(t *testing.T) {
	unittest.RunWithTempDir(t, func(base string) {
		srcDir := filepath.Join(base, "src")
		dstDir := filepath.Join(base, "dst")
		require.NoError(t, os.MkdirAll(srcDir, 0755))

		srcName := "checkpoint.00000003"
		dstName := "checkpoint.00000009"

		// Create all files except the header.
		paths := wal.CheckpointV6AllFilePaths(srcDir, srcName)
		for _, p := range paths[1:] { // skip index 0 (header)
			f, err := os.Create(p)
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}

		err := common.MoveCheckpointFiles(srcDir, srcName, dstDir, dstName)
		require.Error(t, err, "expected error when header file is missing")
		require.True(t, errors.Is(err, common.ErrCheckpointFileMissing), "expected ErrCheckpointFileMissing")

		// Destination directory must not have been created.
		_, statErr := os.Stat(dstDir)
		require.True(t, os.IsNotExist(statErr), "destination directory must not be created on validation failure")
	})
}

// TestMoveCheckpointFiles_DestinationExists verifies that the function returns an error
// without moving any files when any destination checkpoint file already exists.
func TestMoveCheckpointFiles_DestinationExists(t *testing.T) {
	unittest.RunWithTempDir(t, func(base string) {
		srcDir := filepath.Join(base, "src")
		dstDir := filepath.Join(base, "dst")
		require.NoError(t, os.MkdirAll(srcDir, 0755))
		require.NoError(t, os.MkdirAll(dstDir, 0755))

		srcName := "checkpoint.00000003"
		dstName := "checkpoint.00000009"

		createFakeCheckpoint(t, srcDir, srcName)
		createFakeCheckpoint(t, dstDir, dstName)

		err := common.MoveCheckpointFiles(srcDir, srcName, dstDir, dstName)
		require.Error(t, err, "expected error when destination file already exists")

		// All source files must remain untouched.
		for i, src := range wal.CheckpointV6AllFilePaths(srcDir, srcName) {
			_, statErr := os.Stat(src)
			require.NoError(t, statErr, "source file %d must remain after failed move: %s", i, src)
		}
	})
}

// TestMoveCheckpointFiles_CreatesDestDir verifies that the destination directory is
// created if it does not already exist.
func TestMoveCheckpointFiles_CreatesDestDir(t *testing.T) {
	unittest.RunWithTempDir(t, func(base string) {
		srcDir := filepath.Join(base, "src")
		// Use a deeply nested path that does not yet exist.
		dstDir := filepath.Join(base, "a", "b", "dst")
		require.NoError(t, os.MkdirAll(srcDir, 0755))

		srcName := "checkpoint.00000001"
		dstName := "checkpoint.00000002"

		createFakeCheckpoint(t, srcDir, srcName)

		require.NoError(t, common.MoveCheckpointFiles(srcDir, srcName, dstDir, dstName))

		info, err := os.Stat(dstDir)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})
}

// TestMoveCheckpointFiles_SameDirRename verifies moving within the same directory (rename).
func TestMoveCheckpointFiles_SameDirRename(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		srcName := "checkpoint.00000004"
		dstName := "checkpoint.00000006"

		createFakeCheckpoint(t, dir, srcName)

		require.NoError(t, common.MoveCheckpointFiles(dir, srcName, dir, dstName))

		for i, dst := range wal.CheckpointV6AllFilePaths(dir, dstName) {
			_, err := os.Stat(dst)
			require.NoError(t, err, "expected renamed file %d: %s", i, dst)
		}

		for i, src := range wal.CheckpointV6AllFilePaths(dir, srcName) {
			_, err := os.Stat(src)
			require.True(t, os.IsNotExist(err), "source file %d must be gone after rename: %s", i, src)
		}
	})
}

// TestCheckpointV6AllFilePaths_Count verifies the function returns exactly 18 paths.
func TestCheckpointV6AllFilePaths_Count(t *testing.T) {
	paths := wal.CheckpointV6AllFilePaths("/some/dir", "checkpoint.00000001")
	require.Len(t, paths, 18)
}

// TestCheckpointV6AllFilePaths_Suffixes verifies the returned paths follow the expected
// naming scheme: header has no suffix, part files end in .000–.016.
func TestCheckpointV6AllFilePaths_Suffixes(t *testing.T) {
	dir := "/data"
	name := "checkpoint.00000001"
	paths := wal.CheckpointV6AllFilePaths(dir, name)

	// Index 0 is the header — no numeric suffix.
	require.Equal(t, "/data/checkpoint.00000001", paths[0])

	// Indices 1–17 are part files .000 through .016.
	for i := 1; i <= 17; i++ {
		expected := fmt.Sprintf("/data/checkpoint.00000001.%03d", i-1)
		require.Equal(t, expected, paths[i], "part file path mismatch at index %d", i)
	}
}
