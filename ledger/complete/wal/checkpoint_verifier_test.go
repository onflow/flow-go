package wal

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestVerifyCheckpointHashesV6 verifies a valid V6 checkpoint at several worker counts.
func TestVerifyCheckpointHashesV6(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		const fileName = "checkpoint"
		tries := createMultipleRandomTries(t)
		require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, zerolog.Nop()))

		for _, nWorker := range []uint{1, 8, 16} {
			require.NoError(t, VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, nWorker))
		}
	})
}

// TestVerifyCheckpointHashesV7 verifies a valid V7 (payloadless) checkpoint.
func TestVerifyCheckpointHashesV7(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		fileName := "checkpoint" + V7FileSuffix
		tries := createMultiplePayloadlessTries(t)
		require.NoError(t, StoreCheckpointV7Concurrently(tries, dir, fileName, zerolog.Nop()))

		for _, nWorker := range []uint{1, 8, 16} {
			require.NoError(t, VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, nWorker))
		}
	})
}

// TestVerifyCheckpointHashesWorkerRange verifies nWorker outside [1, subtrieCount] is rejected.
func TestVerifyCheckpointHashesWorkerRange(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		const fileName = "checkpoint"
		tries := createSimpleTrie(t)
		require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, zerolog.Nop()))

		require.Error(t, VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, 0))
		require.Error(t, VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, subtrieCount+1))
		require.NoError(t, VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, subtrieCount))
	})
}

// TestVerifyCheckpointHashesDetectsCorruption verifies that corrupting a node in a
// subtrie part file is detected as a hash mismatch.
func TestVerifyCheckpointHashesDetectsCorruption(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		const fileName = "checkpoint"
		tries := createMultipleRandomTries(t)
		require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, zerolog.Nop()))

		// Find a non-empty subtrie part file and flip a byte inside the first node's
		// encoding (just past the 4-byte magic+version header).
		corrupted := false
		for i := range subtrieCount {
			partPath, _, err := filePathSubTries(dir, fileName, i)
			require.NoError(t, err)

			info, err := os.Stat(partPath)
			require.NoError(t, err)
			// Skip empty subtries (header + footer only carry no node bytes worth flipping).
			if info.Size() < 64 {
				continue
			}

			flipByteInFile(t, partPath, 8)
			corrupted = true
			break
		}
		require.True(t, corrupted, "expected at least one non-empty subtrie part file")

		err := VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, 16)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrCheckpointHashMismatch)
	})
}

// TestVerifyCheckpointHashesRejectsOutOfRangeHeight verifies that a node whose
// stored height exceeds ledger.NodeMaxHeight is reported as an integrity violation
// instead of panicking while indexing the default-hash table.
func TestVerifyCheckpointHashesRejectsOutOfRangeHeight(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		const fileName = "checkpoint"
		tries := createMultipleRandomTries(t)
		require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, zerolog.Nop()))

		// Overwrite the first node's 2-byte height. It sits just past the 4-byte
		// magic+version header and the 1-byte node type.
		corrupted := false
		for i := range subtrieCount {
			partPath, _, err := filePathSubTries(dir, fileName, i)
			require.NoError(t, err)

			info, err := os.Stat(partPath)
			require.NoError(t, err)
			if info.Size() < 64 {
				continue
			}

			writeBytesInFile(t, partPath, 5, []byte{0xFF, 0xFF})
			corrupted = true
			break
		}
		require.True(t, corrupted, "expected at least one non-empty subtrie part file")

		err := VerifyCheckpointHashes(zerolog.Nop(), dir, fileName, 16)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrCheckpointIntegrity)
	})
}

// flipByteInFile flips one bit of the byte at the given offset in the file.
func flipByteInFile(t *testing.T, path string, offset int64) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	buf := make([]byte, 1)
	_, err = f.ReadAt(buf, offset)
	require.NoError(t, err)

	buf[0] ^= 0xFF
	_, err = f.WriteAt(buf, offset)
	require.NoError(t, err)
}

// writeBytesInFile overwrites len(data) bytes at the given offset in the file.
func writeBytesInFile(t *testing.T, path string, offset int64, data []byte) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	_, err = f.WriteAt(data, offset)
	require.NoError(t, err)
}
