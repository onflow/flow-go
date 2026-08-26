package common_test

import (
	"testing"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	flowWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/utils/unittest"
)

// testSegmentSize is one page (32 KB), the minimum valid segment size for the WAL library.
// With payloads of ~25 KB each record barely fits in one page, so writing N records
// produces N WAL segments — giving us multi-segment fixtures without writing huge amounts
// of test data.
const testSegmentSize = 32 * 1024

// singleSegmentSize is large enough (32 MB) to hold many small records in one segment.
const singleSegmentSize = 32 * 1024 * 1024

func makeRootHash(b byte) ledger.RootHash {
	var h ledger.RootHash
	h[0] = b
	return h
}

// makeTrieUpdate returns a TrieUpdate whose encoded size is large enough (≈ 25 KB) to
// occupy one WAL page on its own, which forces a new segment for the next write when
// testSegmentSize is used.
func makeTrieUpdate(rootHash ledger.RootHash) *ledger.TrieUpdate {
	path := testutils.PathByUint16(0)
	value := make(ledger.Value, 25*1024) // ~25 KB, fills one 32-KB page
	payload := ledger.NewPayload(ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: []byte{1}}}}, value)
	return &ledger.TrieUpdate{
		RootHash: rootHash,
		Paths:    []ledger.Path{path},
		Payloads: []*ledger.Payload{payload},
	}
}

// writeWALUpdate encodes and appends a trie-update record with the given root hash to w.
func writeWALUpdate(t *testing.T, w *prometheusWAL.WAL, rootHash ledger.RootHash) {
	t.Helper()
	update := makeTrieUpdate(rootHash)
	_, err := w.Log(flowWAL.EncodeUpdate(update))
	require.NoError(t, err)
}

// openWALWriter returns a WAL writer using the provided segment size.
func openWALWriter(t *testing.T, dir string, segSize int) *prometheusWAL.WAL {
	t.Helper()
	w, err := prometheusWAL.NewSize(zerolog.Nop(), nil, dir, segSize, false)
	require.NoError(t, err)
	return w
}

// makeSmallTrieUpdate returns a TrieUpdate with a single tiny path/payload pair.
// The encoder skips pathSize when numOfPaths==0, but the decoder always reads it,
// so a TrieUpdate with 0 paths cannot be round-tripped via EncodeUpdate/Decode.
// Using one path+payload avoids that asymmetry.
func makeSmallTrieUpdate(rootHash ledger.RootHash) *ledger.TrieUpdate {
	path := testutils.PathByUint16(0)
	value := make(ledger.Value, 8) // 8-byte value — tiny enough to fit many records per segment
	payload := ledger.NewPayload(ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: []byte{1}}}}, value)
	return &ledger.TrieUpdate{
		RootHash: rootHash,
		Paths:    []ledger.Path{path},
		Payloads: []*ledger.Payload{payload},
	}
}

// writeSmallWALUpdate writes a small (single tiny payload) trie-update record.
func writeSmallWALUpdate(t *testing.T, w *prometheusWAL.WAL, rootHash ledger.RootHash) {
	t.Helper()
	_, err := w.Log(flowWAL.EncodeUpdate(makeSmallTrieUpdate(rootHash)))
	require.NoError(t, err)
}

// TestSearchRootHashForward_SingleSegment verifies that a forward scan finds the target
// hash within a single WAL segment containing multiple records.
func TestSearchRootHashForward_SingleSegment(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hash2 := makeRootHash(0x02)
		hash3 := makeRootHash(0x03)

		// Use a large segment so all three tiny records land in segment 0.
		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		writeSmallWALUpdate(t, w, hash2)
		writeSmallWALUpdate(t, w, hash3)
		require.NoError(t, w.Close())

		seg, _, err := common.SearchRootHashForward(hash2, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)
		require.Equal(t, 0, seg)
	})
}

// TestSearchRootHashForward_NotFound verifies that a forward scan returns an error when
// the target hash is absent from the WAL.
func TestSearchRootHashForward_NotFound(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hashMissing := makeRootHash(0xFF)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		require.NoError(t, w.Close())

		_, _, err := common.SearchRootHashForward(hashMissing, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.Error(t, err)
	})
}

// TestSearchRootHashBackward_SingleSegment verifies that a backward scan finds the target
// hash within a single WAL segment containing multiple records.
func TestSearchRootHashBackward_SingleSegment(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hash2 := makeRootHash(0x02)

		// Use a large segment so both tiny records land in segment 0.
		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		writeSmallWALUpdate(t, w, hash2)
		require.NoError(t, w.Close())

		seg, _, err := common.SearchRootHashBackward(hash1, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)
		require.Equal(t, 0, seg)
	})
}

// TestSearchRootHashBackward_NotFound verifies that a backward scan returns an error when
// the target hash is absent from the WAL.
func TestSearchRootHashBackward_NotFound(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hashMissing := makeRootHash(0xFF)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		require.NoError(t, w.Close())

		_, _, err := common.SearchRootHashBackward(hashMissing, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.Error(t, err)
	})
}

// TestSearchRootHashBackward_MultipleSegments is the core correctness test for the
// backward scan. It writes target hashes across three separate WAL segments and verifies:
//
//  1. The backward scan finds the hash in the LAST segment that contains it, not the first.
//  2. The backward scan returns the correct segment number in all positions (first, middle, last).
//  3. When the same hash appears in multiple segments, the backward scan returns the
//     segment with the highest index (the most recent occurrence).
func TestSearchRootHashBackward_MultipleSegments(t *testing.T) {
	// Each call to writeWALUpdate writes a ~25 KB record.  With testSegmentSize = 32 KB
	// the WAL rotates to a new segment after each write, so the segment index equals the
	// write index (0-based).
	unittest.RunWithTempDir(t, func(dir string) {
		hashA := makeRootHash(0xAA)
		hashB := makeRootHash(0xBB)
		hashC := makeRootHash(0xCC)

		w := openWALWriter(t, dir, testSegmentSize)
		writeWALUpdate(t, w, hashA) // segment 0
		writeWALUpdate(t, w, hashB) // segment 1
		writeWALUpdate(t, w, hashC) // segment 2
		require.NoError(t, w.Close())

		from, to, err := prometheusWAL.Segments(dir)
		require.NoError(t, err)
		require.Equal(t, 0, from)
		require.Equal(t, 2, to, "expected 3 segments (0–2)")

		t.Run("hash in first segment", func(t *testing.T) {
			seg, _, err := common.SearchRootHashBackward(hashA, dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.NoError(t, err)
			require.Equal(t, 0, seg)
		})

		t.Run("hash in middle segment", func(t *testing.T) {
			seg, _, err := common.SearchRootHashBackward(hashB, dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.NoError(t, err)
			require.Equal(t, 1, seg)
		})

		t.Run("hash in last segment", func(t *testing.T) {
			seg, _, err := common.SearchRootHashBackward(hashC, dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.NoError(t, err)
			require.Equal(t, 2, seg)
		})

		t.Run("hash not present in any segment", func(t *testing.T) {
			_, _, err := common.SearchRootHashBackward(makeRootHash(0xFF), dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.Error(t, err)
		})
	})
}

// TestSearchRootHashBackward_ReturnsLastOccurrence verifies that when the same root hash
// appears in multiple segments the backward scan returns the highest (most recent) segment.
func TestSearchRootHashBackward_ReturnsLastOccurrence(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hashDup := makeRootHash(0xDD)
		hashOther := makeRootHash(0xEE)

		w := openWALWriter(t, dir, testSegmentSize)
		writeWALUpdate(t, w, hashDup)   // segment 0 — first occurrence
		writeWALUpdate(t, w, hashOther) // segment 1 — other hash
		writeWALUpdate(t, w, hashDup)   // segment 2 — second (later) occurrence
		require.NoError(t, w.Close())

		seg, _, err := common.SearchRootHashBackward(hashDup, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)
		require.Equal(t, 2, seg, "backward scan should return the most recent segment")
	})
}

// TestSearchRootHashBackward_BoundedRange verifies that the wantFrom/wantTo range bounds
// are respected: a hash outside the bounded range must not be found.
func TestSearchRootHashBackward_BoundedRange(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hashA := makeRootHash(0x11)
		hashB := makeRootHash(0x22)
		hashC := makeRootHash(0x33)

		w := openWALWriter(t, dir, testSegmentSize)
		writeWALUpdate(t, w, hashA) // segment 0
		writeWALUpdate(t, w, hashB) // segment 1
		writeWALUpdate(t, w, hashC) // segment 2
		require.NoError(t, w.Close())

		// Restrict search to segment 1 only.
		seg, _, err := common.SearchRootHashBackward(hashB, dir, 1, 1)
		require.NoError(t, err)
		require.Equal(t, 1, seg)

		// hashA is only in segment 0, which is outside the bounded range [1,1].
		_, _, err = common.SearchRootHashBackward(hashA, dir, 1, 1)
		require.Error(t, err, "hash outside the bounded range must not be found")
	})
}
