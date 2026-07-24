package compare_cadence_vm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func testRunConfig(blockIDs ...flow.Identifier) runConfig {
	return runConfig{
		Chain:        string(flow.Mainnet),
		BlockIDs:     blockIDStrings(blockIDs),
		BlockCount:   100,
		ComputeLimit: 9999,
	}
}

func TestWriteAndReadRunProgress(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "progress.json")

	startBlockID := unittest.IdentifierFixture()
	nextBlockID := unittest.IdentifierFixture()

	progress := newRunProgress(testRunConfig(startBlockID), startBlockID)
	progress.CompletedBlockCount = 10
	progress.NextBlockID = nextBlockID.String()
	progress.Stats = runStats{
		BlocksMatched:          9,
		BlocksMismatched:       1,
		TransactionsMatched:    40,
		TransactionsMismatched: 2,
	}

	require.NoError(t, writeRunProgress(path, progress))

	read, err := readRunProgress(path)
	require.NoError(t, err)
	assert.Equal(t, progress, read)

	// Writing again must replace the previously recorded progress.
	progress.CompletedBlockCount = 20
	require.NoError(t, writeRunProgress(path, progress))

	read, err = readRunProgress(path)
	require.NoError(t, err)
	assert.Equal(t, progress, read)

	// Writing must not leave temporary files behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Base(path), entries[0].Name())
}

func TestReadRunProgressRejectsUnexpectedContents(t *testing.T) {
	t.Parallel()

	t.Run("unknown field", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "progress.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": 1, "unexpected": 1}`), 0644))

		_, err := readRunProgress(path)
		require.ErrorContains(t, err, "failed to decode progress file")
	})

	t.Run("trailing value", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "progress.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": 1}{"version": 1}`), 0644))

		_, err := readRunProgress(path)
		require.ErrorContains(t, err, "more than the expected progress")
	})
}

func TestValidateRunProgress(t *testing.T) {
	t.Parallel()

	startBlockID := unittest.IdentifierFixture()
	nextBlockID := unittest.IdentifierFixture()
	config := testRunConfig(startBlockID)

	newProgress := func() runProgress {
		progress := newRunProgress(config, startBlockID)
		progress.CompletedBlockCount = 10
		progress.NextBlockID = nextBlockID.String()
		return progress
	}

	t.Run("resumable", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, newProgress().validate(config, 100))
	})

	t.Run("completed run", func(t *testing.T) {
		t.Parallel()

		progress := newProgress()
		progress.CompletedBlockCount = 100
		progress.NextBlockID = ""

		require.NoError(t, progress.validate(config, 100))
	})

	t.Run("different version", func(t *testing.T) {
		t.Parallel()

		progress := newProgress()
		progress.Version = progressVersion + 1

		require.ErrorContains(t, progress.validate(config, 100), "version")
	})

	t.Run("different compute limit", func(t *testing.T) {
		t.Parallel()

		otherConfig := config
		otherConfig.ComputeLimit = config.ComputeLimit + 1

		require.ErrorContains(t, newProgress().validate(otherConfig, 100), "different configuration")
	})

	t.Run("different block IDs", func(t *testing.T) {
		t.Parallel()

		otherConfig := config
		otherConfig.BlockIDs = blockIDStrings([]flow.Identifier{unittest.IdentifierFixture()})

		require.ErrorContains(t, newProgress().validate(otherConfig, 100), "different configuration")
	})

	t.Run("more compared blocks than the run has", func(t *testing.T) {
		t.Parallel()

		progress := newProgress()
		progress.CompletedBlockCount = 101

		require.ErrorContains(t, progress.validate(config, 100), "outside the 100 blocks")
	})

	t.Run("missing next block ID", func(t *testing.T) {
		t.Parallel()

		progress := newProgress()
		progress.NextBlockID = ""

		require.ErrorContains(t, progress.validate(config, 100), "does not contain the ID of the next block")
	})

	t.Run("invalid next block ID", func(t *testing.T) {
		t.Parallel()

		progress := newProgress()
		progress.NextBlockID = "not-a-block-ID"

		require.ErrorContains(t, progress.validate(config, 100), "invalid next block ID")
	})
}

func TestBlockLoaderAdvance(t *testing.T) {
	t.Parallel()

	t.Run("follows parents", func(t *testing.T) {
		t.Parallel()

		header := unittest.BlockHeaderFixture()

		loader := &blockLoader{
			nextBlockID: header.ID(),
		}
		loader.loadedBlockCount++
		loader.advance(header)

		assert.Equal(t, header.ParentID, loader.nextBlockID)
	})

	t.Run("explicit block IDs", func(t *testing.T) {
		t.Parallel()

		blockIDs := unittest.IdentifierListFixture(2)

		loader := &blockLoader{
			explicitBlockIDs: blockIDs,
			nextBlockID:      blockIDs[0],
		}

		// The parent of the loaded block must be ignored, the explicit blocks are unrelated.
		header := unittest.BlockHeaderFixture()

		loader.loadedBlockCount++
		loader.advance(header)
		assert.Equal(t, blockIDs[1], loader.nextBlockID)

		loader.loadedBlockCount++
		loader.advance(header)
		assert.Equal(t, flow.ZeroID, loader.nextBlockID)
	})
}

func TestResolveProgressFilePath(t *testing.T) {
	// The resolved path depends on the flags, which are package level variables,
	// so these cases must not run in parallel.

	blockID := unittest.IdentifierFixture()
	defaultName := defaultProgressFileName(blockID, 100)

	tests := []struct {
		name         string
		progressFile string
		batchSize    int
		resume       bool
		expected     string
	}{
		{
			name:         "explicit progress file",
			progressFile: "/tmp/progress.json",
			expected:     "/tmp/progress.json",
		},
		{
			name:      "batched run without progress file",
			batchSize: 10,
			expected:  defaultName,
		},
		{
			name:     "resumed run without progress file",
			resume:   true,
			expected: defaultName,
		},
		{
			name:         "explicit progress file takes precedence",
			progressFile: "/tmp/progress.json",
			batchSize:    10,
			expected:     "/tmp/progress.json",
		},
		{
			// A run which compares all blocks in a single batch has no progress to record.
			name:     "single batch without progress file",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previousProgressFile := flagProgressFile
			previousBatchSize := flagBatchSize
			previousResume := flagResume
			t.Cleanup(func() {
				flagProgressFile = previousProgressFile
				flagBatchSize = previousBatchSize
				flagResume = previousResume
			})

			flagProgressFile = test.progressFile
			flagBatchSize = test.batchSize
			flagResume = test.resume

			assert.Equal(t, test.expected, resolveProgressFilePath(blockID, 100))
		})
	}
}

func TestDefaultProgressFileName(t *testing.T) {
	t.Parallel()

	blockID := unittest.IdentifierFixture()

	name := defaultProgressFileName(blockID, 100)

	// The name must identify the run, so that the run resumes from its own progress file.
	assert.Equal(
		t,
		fmt.Sprintf("compare-cadence-vm-%s-100.progress.json", blockID.String()[:16]),
		name,
	)

	// The name must be a file in the directory the command is run in.
	assert.Equal(t, name, filepath.Base(name))

	assert.NotEqual(t, name, defaultProgressFileName(blockID, 200))
	assert.NotEqual(t, name, defaultProgressFileName(unittest.IdentifierFixture(), 100))
}

func TestBlockIDString(t *testing.T) {
	t.Parallel()

	blockID := unittest.IdentifierFixture()
	assert.Equal(t, blockID.String(), blockIDString(blockID))

	// The zero identifier means that no block is left to compare.
	assert.Equal(t, "", blockIDString(flow.ZeroID))
}
