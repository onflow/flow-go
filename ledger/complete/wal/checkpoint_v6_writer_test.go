package wal

import (
	"os"
	"path"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestRemoveStaleTempFiles verifies that removeStaleTempFiles deletes only the
// "writing-<outputFile>*" temp files for the given output, while leaving final
// part files, the header, and temp files belonging to other outputs untouched.
func TestRemoveStaleTempFiles(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		outputFile := "root.checkpoint.v7"

		// Stale temp files for outputFile: subtries, top-trie, and header.
		// These mirror the names produced by createClosableWriter
		// ("writing-<fileName>-<random>").
		staleTempFiles := []string{
			"writing-root.checkpoint.v7.000-1234567890",
			"writing-root.checkpoint.v7.000-9876543210", // a second orphan for the same part
			"writing-root.checkpoint.v7.016-1720029787", // top-trie part
			"writing-root.checkpoint.v7-246069680",      // header
		}

		// Files that must NOT be removed: final part files, the header, and a temp
		// file for a different output (e.g. a V6 checkpoint with a different name).
		keepFiles := []string{
			"root.checkpoint.v7",                       // final header
			"root.checkpoint.v7.000",                   // final subtrie part
			"root.checkpoint.v7.016",                   // final top-trie part
			"writing-root.checkpoint.v6.000-111222333", // temp for a different output
			"root.checkpoint.v6",                       // unrelated final file
		}

		for _, name := range append(append([]string{}, staleTempFiles...), keepFiles...) {
			require.NoError(t, os.WriteFile(path.Join(dir, name), []byte("x"), 0644))
		}

		require.NoError(t, removeStaleTempFiles(dir, outputFile, zerolog.Nop()))

		for _, name := range staleTempFiles {
			require.NoFileExists(t, path.Join(dir, name), "stale temp file should have been removed: %s", name)
		}
		for _, name := range keepFiles {
			require.FileExists(t, path.Join(dir, name), "file should have been kept: %s", name)
		}
	})
}

// TestRemoveStaleTempFiles_NoMatches verifies that removeStaleTempFiles is a
// no-op (no error) when there are no matching temp files.
func TestRemoveStaleTempFiles_NoMatches(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		require.NoError(t, os.WriteFile(path.Join(dir, "root.checkpoint.v7.000"), []byte("x"), 0644))

		require.NoError(t, removeStaleTempFiles(dir, "root.checkpoint.v7", zerolog.Nop()))

		require.FileExists(t, path.Join(dir, "root.checkpoint.v7.000"))
	})
}
