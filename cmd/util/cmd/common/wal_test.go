package common_test

import (
	"crypto/rand"
	"errors"
	"os"
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

// makeLargeTrieUpdate returns a TrieUpdate whose encoded size is larger than one WAL page,
// forcing the WAL writer to split the record across multiple pages.
func makeLargeTrieUpdate(rootHash ledger.RootHash) *ledger.TrieUpdate {
	path := testutils.PathByUint16(0)
	value := make(ledger.Value, 100*1024) // incompressible payload > 32 KB, spans multiple pages
	_, err := rand.Read(value)
	if err != nil {
		panic(err)
	}
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

// writeLargeWALUpdate encodes and appends a multi-page trie-update record with the given root hash to w.
func writeLargeWALUpdate(t *testing.T, w *prometheusWAL.WAL, rootHash ledger.RootHash) {
	t.Helper()
	update := makeLargeTrieUpdate(rootHash)
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

		seg, _, err := common.SearchRootHashForward(zerolog.Nop(), hash2, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)
		require.Equal(t, 0, seg)
	})
}

// TestSearchRootHashForward_NotFound verifies that a forward scan returns an expected
// error when the target hash is absent from the WAL.
func TestSearchRootHashForward_NotFound(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hashMissing := makeRootHash(0xFF)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		require.NoError(t, w.Close())

		_, _, err := common.SearchRootHashForward(zerolog.Nop(), hashMissing, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.Error(t, err)
		require.ErrorIs(t, err, common.ErrRootHashNotFound)
	})
}

// TestSearchRootHashForward_InvalidRange verifies that an inverted range is rejected
// as an expected error before any WAL access happens.
func TestSearchRootHashForward_InvalidRange(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		_, _, err := common.SearchRootHashForward(zerolog.Nop(), makeRootHash(0x01), dir, 5, 2)
		require.Error(t, err)
		require.ErrorIs(t, err, common.ErrInvalidSegmentRange)
	})
}

// TestSearchRootHashForward_CorruptFinalRecord verifies that a forward scan returns an
// exception (not a successful match) when the final WAL record is torn/corrupt.
func TestSearchRootHashForward_CorruptFinalRecord(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hash2 := makeRootHash(0x02)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		writeLargeWALUpdate(t, w, hash2)
		require.NoError(t, w.Close())

		truncateLastSegmentToPages(t, dir, 2)

		_, _, err := common.SearchRootHashForward(zerolog.Nop(), hash1, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.Error(t, err)
		require.False(t, errors.Is(err, common.ErrRootHashNotFound), "corrupt WAL must not be reported as not found")
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

		seg, _, err := common.SearchRootHashBackward(zerolog.Nop(), hash1, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)
		require.Equal(t, 0, seg)
	})
}

// TestSearchRootHashBackward_NotFound verifies that a backward scan returns an expected
// error when the target hash is absent from the WAL.
func TestSearchRootHashBackward_NotFound(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hashMissing := makeRootHash(0xFF)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		require.NoError(t, w.Close())

		_, _, err := common.SearchRootHashBackward(zerolog.Nop(), hashMissing, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.Error(t, err)
		require.ErrorIs(t, err, common.ErrRootHashNotFound)
	})
}

// TestSearchRootHashBackward_InvalidRange verifies that an inverted range is rejected
// as an expected error before any WAL access happens.
func TestSearchRootHashBackward_InvalidRange(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		_, _, err := common.SearchRootHashBackward(zerolog.Nop(), makeRootHash(0x01), dir, 5, 2)
		require.Error(t, err)
		require.ErrorIs(t, err, common.ErrInvalidSegmentRange)
	})
}

// TestSearchRootHashBackward_CorruptFinalRecord verifies that a backward scan returns an
// exception when the last segment has a torn final record, even if an earlier segment
// contains a matching hash.
func TestSearchRootHashBackward_CorruptFinalRecord(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hash1 := makeRootHash(0x01)
		hash2 := makeRootHash(0x02)

		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hash1)
		writeLargeWALUpdate(t, w, hash2)
		require.NoError(t, w.Close())

		truncateLastSegmentToPages(t, dir, 2)

		_, _, err := common.SearchRootHashBackward(zerolog.Nop(), hash1, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.Error(t, err)
		require.False(t, errors.Is(err, common.ErrRootHashNotFound), "corrupt WAL must not be reported as not found")
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
			seg, _, err := common.SearchRootHashBackward(zerolog.Nop(), hashA, dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.NoError(t, err)
			require.Equal(t, 0, seg)
		})

		t.Run("hash in middle segment", func(t *testing.T) {
			seg, _, err := common.SearchRootHashBackward(zerolog.Nop(), hashB, dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.NoError(t, err)
			require.Equal(t, 1, seg)
		})

		t.Run("hash in last segment", func(t *testing.T) {
			seg, _, err := common.SearchRootHashBackward(zerolog.Nop(), hashC, dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.NoError(t, err)
			require.Equal(t, 2, seg)
		})

		t.Run("hash not present in any segment", func(t *testing.T) {
			_, _, err := common.SearchRootHashBackward(zerolog.Nop(), makeRootHash(0xFF), dir, common.DefaultWALFrom, common.DefaultWALTo)
			require.Error(t, err)
			require.ErrorIs(t, err, common.ErrRootHashNotFound)
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

		seg, _, err := common.SearchRootHashBackward(zerolog.Nop(), hashDup, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)
		require.Equal(t, 2, seg, "backward scan should return the most recent segment")
	})
}

// TestSearchRootHashBackward_ReturnsLastOffsetInSegment verifies that when the same root
// hash appears twice within one segment, the backward scan returns the offset of the
// last occurrence, not the first.
func TestSearchRootHashBackward_ReturnsLastOffsetInSegment(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		hashDup := makeRootHash(0xDD)
		hashOther := makeRootHash(0xEE)

		// Use a large segment so all records land in segment 0.
		w := openWALWriter(t, dir, singleSegmentSize)
		writeSmallWALUpdate(t, w, hashDup)   // offset 0
		writeSmallWALUpdate(t, w, hashOther) // offset 1
		writeSmallWALUpdate(t, w, hashDup)   // offset 2 — last occurrence
		require.NoError(t, w.Close())

		_, offset, err := common.SearchRootHashBackward(zerolog.Nop(), hashDup, dir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)

		lastOffset := lastOffsetOfHashInSegment(t, dir, 0, hashDup)
		require.Equal(t, lastOffset, offset, "backward scan should return the last in-segment offset")
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
		seg, _, err := common.SearchRootHashBackward(zerolog.Nop(), hashB, dir, 1, 1)
		require.NoError(t, err)
		require.Equal(t, 1, seg)

		// hashA is only in segment 0, which is outside the bounded range [1,1].
		_, _, err = common.SearchRootHashBackward(zerolog.Nop(), hashA, dir, 1, 1)
		require.Error(t, err, "hash outside the bounded range must not be found")
		require.ErrorIs(t, err, common.ErrRootHashNotFound)
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

	return lastOffset
}

// truncateLastSegmentToPages truncates the last segment in dir to exactly pageCount pages.
func truncateLastSegmentToPages(t *testing.T, dir string, pageCount int) {
	t.Helper()

	first, last, err := prometheusWAL.Segments(dir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, last, first)

	segPath := prometheusWAL.SegmentName(dir, last)
	err = os.Truncate(segPath, int64(pageCount*testSegmentSize))
	require.NoError(t, err)
}
