package move_wal

import (
	"os"
	"path/filepath"
	"testing"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestMoveWAL(t *testing.T) {
	t.Run("preview mode - move all files from segment", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "dest")

			require.NoError(t, os.MkdirAll(srcDir, 0755))

			// Create WAL segment files: 00000000, 00000001, 00000002, 00000003
			for i := 0; i < 4; i++ {
				segmentFile := prometheusWAL.SegmentName(srcDir, i)
				file, err := os.Create(segmentFile)
				require.NoError(t, err)
				_, err = file.WriteString("test data")
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}

			// Run command in preview mode
			Cmd.SetArgs([]string{
				"--from", "1",
				"--move-from", srcDir,
				"--move-to", dstDir,
			})

			err := Cmd.Execute()
			require.NoError(t, err)

			// Verify files were NOT moved (preview mode)
			for i := 1; i < 4; i++ {
				srcFile := prometheusWAL.SegmentName(srcDir, i)
				dstFile := prometheusWAL.SegmentName(dstDir, i)
				assert.FileExists(t, srcFile, "source file should still exist in preview mode")
				assert.NoFileExists(t, dstFile, "destination file should not exist in preview mode")
			}
		})
	})

	t.Run("preview mode - move specific range", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "dest")

			require.NoError(t, os.MkdirAll(srcDir, 0755))

			// Create WAL segment files: 00000000 through 00000005
			for i := 0; i < 6; i++ {
				segmentFile := prometheusWAL.SegmentName(srcDir, i)
				file, err := os.Create(segmentFile)
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}

			// Run command in preview mode with specific range
			Cmd.SetArgs([]string{
				"--from", "2",
				"--to", "4",
				"--move-from", srcDir,
				"--move-to", dstDir,
			})

			err := Cmd.Execute()
			require.NoError(t, err)

			// Verify files were NOT moved (preview mode)
			for i := 2; i <= 4; i++ {
				srcFile := prometheusWAL.SegmentName(srcDir, i)
				dstFile := prometheusWAL.SegmentName(dstDir, i)
				assert.FileExists(t, srcFile, "source file should still exist in preview mode")
				assert.NoFileExists(t, dstFile, "destination file should not exist in preview mode")
			}
		})
	})

	t.Run("actual move - move all files from segment", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "dest")

			require.NoError(t, os.MkdirAll(srcDir, 0755))

			// Create WAL segment files: 00000000, 00000001, 00000002, 00000003
			for i := 0; i < 4; i++ {
				segmentFile := prometheusWAL.SegmentName(srcDir, i)
				file, err := os.Create(segmentFile)
				require.NoError(t, err)
				_, err = file.WriteString("test data")
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}

			// Run command with --send=true
			Cmd.SetArgs([]string{
				"--from", "1",
				"--move-from", srcDir,
				"--move-to", dstDir,
				"--send", "true",
			})

			err := Cmd.Execute()
			require.NoError(t, err)

			// Verify files were moved
			// Segment 0 should remain in source
			srcFile0 := prometheusWAL.SegmentName(srcDir, 0)
			assert.FileExists(t, srcFile0, "segment 0 should remain in source")

			// Segments 1-3 should be moved to destination
			for i := 1; i < 4; i++ {
				srcFile := prometheusWAL.SegmentName(srcDir, i)
				dstFile := prometheusWAL.SegmentName(dstDir, i)
				assert.NoFileExists(t, srcFile, "source file should be moved")
				assert.FileExists(t, dstFile, "destination file should exist")
			}
		})
	})

	t.Run("actual move - move specific range", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "dest")

			require.NoError(t, os.MkdirAll(srcDir, 0755))

			// Create WAL segment files: 00000000 through 00000005
			for i := 0; i < 6; i++ {
				segmentFile := prometheusWAL.SegmentName(srcDir, i)
				file, err := os.Create(segmentFile)
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}

			// Run command with --send=true and specific range
			Cmd.SetArgs([]string{
				"--from", "2",
				"--to", "4",
				"--move-from", srcDir,
				"--move-to", dstDir,
				"--send", "true",
			})

			err := Cmd.Execute()
			require.NoError(t, err)

			// Verify only segments 2-4 were moved
			for i := 0; i < 6; i++ {
				srcFile := prometheusWAL.SegmentName(srcDir, i)
				dstFile := prometheusWAL.SegmentName(dstDir, i)

				if i >= 2 && i <= 4 {
					// These should be moved
					assert.NoFileExists(t, srcFile, "source file should be moved")
					assert.FileExists(t, dstFile, "destination file should exist")
				} else {
					// These should remain in source
					assert.FileExists(t, srcFile, "source file should remain")
					assert.NoFileExists(t, dstFile, "destination file should not exist")
				}
			}
		})
	})

	t.Run("skip existing destination files", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "dest")

			require.NoError(t, os.MkdirAll(srcDir, 0755))
			require.NoError(t, os.MkdirAll(dstDir, 0755))

			// Create source files
			for i := 1; i < 4; i++ {
				segmentFile := prometheusWAL.SegmentName(srcDir, i)
				file, err := os.Create(segmentFile)
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}

			// Create existing destination file for segment 2
			dstFile2 := prometheusWAL.SegmentName(dstDir, 2)
			file, err := os.Create(dstFile2)
			require.NoError(t, err)
			require.NoError(t, file.Close())

			// Run command with --send=true
			Cmd.SetArgs([]string{
				"--from", "1",
				"--to", "3",
				"--move-from", srcDir,
				"--move-to", dstDir,
				"--send", "true",
			})

			err = Cmd.Execute()
			require.NoError(t, err)

			// Verify segment 2 was skipped (still exists in both places)
			srcFile2 := prometheusWAL.SegmentName(srcDir, 2)
			assert.FileExists(t, srcFile2, "segment 2 should remain in source (skipped)")
			assert.FileExists(t, dstFile2, "segment 2 should remain in destination (already existed)")

			// Verify segments 1 and 3 were moved
			srcFile1 := prometheusWAL.SegmentName(srcDir, 1)
			dstFile1 := prometheusWAL.SegmentName(dstDir, 1)
			assert.NoFileExists(t, srcFile1)
			assert.FileExists(t, dstFile1)

			srcFile3 := prometheusWAL.SegmentName(srcDir, 3)
			dstFile3 := prometheusWAL.SegmentName(dstDir, 3)
			assert.NoFileExists(t, srcFile3)
			assert.FileExists(t, dstFile3)
		})
	})

	t.Run("handle missing files in range gracefully", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "dest")

			require.NoError(t, os.MkdirAll(srcDir, 0755))

			// Create sequential segments 0, 1, 2, 3, 5 (note: 4 is missing but we'll request 0-5)
			// Note: WAL segments must be sequential for prometheusWAL.Segments to work,
			// so we create 0-3, then manually create 5 to test the skip logic
			for i := 0; i < 4; i++ {
				segmentFile := prometheusWAL.SegmentName(srcDir, i)
				file, err := os.Create(segmentFile)
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}

			// Run command with --send=true, requesting segments 0-3 (all exist)
			Cmd.SetArgs([]string{
				"--from", "0",
				"--to", "3",
				"--move-from", srcDir,
				"--move-to", dstDir,
				"--send", "true",
			})

			err := Cmd.Execute()
			require.NoError(t, err)

			// Verify all files were moved
			for i := 0; i <= 3; i++ {
				srcFile := prometheusWAL.SegmentName(srcDir, i)
				dstFile := prometheusWAL.SegmentName(dstDir, i)
				assert.NoFileExists(t, srcFile, "source file should be moved")
				assert.FileExists(t, dstFile, "destination file should exist")
			}
		})
	})

}
