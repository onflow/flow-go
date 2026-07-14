package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/complete/wal"
	modelbootstrap "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFindLatestCheckpointFilePath verifies that the returned file name matches the version of the
// latest checkpoint in the directory. V6 and V7 checkpoints coexist in a payloadless triedir, so
// rendering the wrong version's name would point at a file that does not exist.
func TestFindLatestCheckpointFilePath(t *testing.T) {
	v6Root := modelbootstrap.FilenameWALRootCheckpoint
	v7Root := modelbootstrap.FilenameWALRootCheckpoint + wal.V7FileSuffix

	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "empty directory falls back to the V6 root checkpoint",
			files:    nil,
			expected: v6Root,
		},
		{
			name:     "V6 root checkpoint only",
			files:    []string{v6Root},
			expected: v6Root,
		},
		{
			name: "V7 root checkpoint is preferred over the V6 root checkpoint",
			// this is the state of a payloadless triedir right after bootstrap: the V6 root
			// checkpoint copied from the bootstrap folder, plus its V7 conversion
			files:    []string{v6Root, v7Root},
			expected: v7Root,
		},
		{
			name:     "numbered V6 checkpoint",
			files:    []string{v6Root, wal.NumberToFilename(10)},
			expected: wal.NumberToFilename(10),
		},
		{
			name:     "numbered V7 checkpoint",
			files:    []string{v6Root, v7Root, wal.NumberToFilenameV7(10)},
			expected: wal.NumberToFilenameV7(10),
		},
		{
			name:     "highest number wins across versions",
			files:    []string{wal.NumberToFilename(20), wal.NumberToFilenameV7(10)},
			expected: wal.NumberToFilename(20),
		},
		{
			name:     "V7 wins over V6 at the same number",
			files:    []string{wal.NumberToFilename(10), wal.NumberToFilenameV7(10)},
			expected: wal.NumberToFilenameV7(10),
		},
		{
			name: "numbered checkpoints win over the root checkpoint",
			files: []string{
				v6Root, v7Root,
				wal.NumberToFilename(10), wal.NumberToFilenameV7(10),
				wal.NumberToFilenameV7(11),
			},
			expected: wal.NumberToFilenameV7(11),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unittest.RunWithTempDir(t, func(dir string) {
				for _, name := range tc.files {
					require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte{}, 0644))
				}

				checkpointFilePath, err := findLatestCheckpointFilePath(dir)
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, tc.expected), checkpointFilePath)
			})
		})
	}
}
