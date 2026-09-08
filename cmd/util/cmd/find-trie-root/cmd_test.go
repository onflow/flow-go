package find_trie_root

import (
	"os"
	"path/filepath"
	"testing"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	flowWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/utils/unittest"
)

const singleSegmentSize = 32 * 1024 * 1024

func makeRootHash(b byte) ledger.RootHash {
	var h ledger.RootHash
	h[0] = b
	return h
}

func makeSmallTrieUpdate(rootHash ledger.RootHash) *ledger.TrieUpdate {
	path := testutils.PathByUint16(0)
	value := make(ledger.Value, 8)
	payload := ledger.NewPayload(ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: []byte{1}}}}, value)
	return &ledger.TrieUpdate{
		RootHash: rootHash,
		Paths:    []ledger.Path{path},
		Payloads: []*ledger.Payload{payload},
	}
}

func writeSmallWALUpdate(t *testing.T, w *prometheusWAL.WAL, rootHash ledger.RootHash) {
	t.Helper()
	_, err := w.Log(flowWAL.EncodeUpdate(makeSmallTrieUpdate(rootHash)))
	require.NoError(t, err)
}

func openWALWriter(t *testing.T, dir string, segSize int) *prometheusWAL.WAL {
	t.Helper()
	w, err := prometheusWAL.NewSize(zerolog.Nop(), nil, dir, segSize, false)
	require.NoError(t, err)
	return w
}

// TestFindRootHashAndCreateTrimmed_StopsAtSelectedOffset verifies that trimming a segment
// with duplicate target hashes stops at the selected offset, not at the first in-segment
// match.
func TestFindRootHashAndCreateTrimmed_StopsAtSelectedOffset(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hashDup := makeRootHash(0xDD)
		hashOther := makeRootHash(0xEE)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hashDup)   // offset 0
		writeSmallWALUpdate(t, w, hashOther) // offset 1
		writeSmallWALUpdate(t, w, hashDup)   // offset 2 — selected offset
		writeSmallWALUpdate(t, w, hashOther) // offset 3 — should be trimmed off
		require.NoError(t, w.Close())

		selectedOffset := lastOffsetOfHashInSegment(t, dir, 0, hashDup)

		tmpDir := filepath.Join(dir, "tmp")
		err := os.Mkdir(tmpDir, 0o700)
		require.NoError(t, err)

		newSegmentFile, err := findRootHashAndCreateTrimmed(dir, 0, selectedOffset, hashDup, tmpDir)
		require.NoError(t, err)

		require.FileExists(t, newSegmentFile)

		// Read the new segment and verify it contains exactly the records up to and
		// including the selected offset.
		segment, err := prometheusWAL.OpenReadSegment(newSegmentFile)
		require.NoError(t, err)
		defer segment.Close()

		reader := prometheusWAL.NewReader(prometheusWAL.NewSegmentBufReader(zerolog.Nop(), segment))
		var seenHashes []ledger.RootHash
		for reader.Next() {
			record := reader.Record()
			op, _, update, err := flowWAL.Decode(record)
			require.NoError(t, err)
			require.Equal(t, flowWAL.WALUpdate, op)
			seenHashes = append(seenHashes, update.RootHash)
		}
		require.NoError(t, reader.Err())

		require.Equal(t, []ledger.RootHash{hashDup, hashOther, hashDup}, seenHashes,
			"trimmed segment must stop at the selected occurrence of the target hash")
	})
}

// lastOffsetOfHashInSegment reads the given segment and returns the offset of the last
// record whose root hash equals expectedHash.
func lastOffsetOfHashInSegment(t *testing.T, dir string, seg int, expectedHash ledger.RootHash) int64 {
	t.Helper()

	segment, err := prometheusWAL.OpenReadSegment(prometheusWAL.SegmentName(dir, seg))
	require.NoError(t, err)
	defer segment.Close()

	reader := prometheusWAL.NewReader(prometheusWAL.NewSegmentBufReader(zerolog.Nop(), segment))
	var lastOffset int64
	for reader.Next() {
		record := reader.Record()
		op, _, update, err := flowWAL.Decode(record)
		require.NoError(t, err)
		if op == flowWAL.WALUpdate && update.RootHash.Equals(expectedHash) {
			lastOffset = reader.Offset()
		}
	}
	require.NoError(t, reader.Err())

	return lastOffset
}
