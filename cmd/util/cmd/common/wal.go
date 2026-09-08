package common

import (
	"errors"
	"fmt"
	"math"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// ErrRootHashNotFound indicates that the searched root hash does not appear in the
// requested WAL segment range.
var ErrRootHashNotFound = errors.New("root hash not found in WAL segments")

// ErrNoWALSegments indicates that the WAL directory contains no segments.
var ErrNoWALSegments = errors.New("no WAL segments found")

// ErrInvalidSegmentRange indicates that the requested [wantFrom, wantTo] bounds are
// invalid or do not overlap the existing WAL segments.
var ErrInvalidSegmentRange = errors.New("invalid WAL segment range")

// SearchRootHashForward scans WAL segments in forward order (first to last) and returns
// the segment index and byte offset of the first occurrence of expectedHash.
//
// wantFrom and wantTo are inclusive bounds on the segment range to search; pass 0 and
// [math.MaxInt32] to search all available segments.
//
// Expected error returns during normal operation:
//   - [ErrRootHashNotFound]: if expectedHash does not appear in the searched segment range.
//   - [ErrNoWALSegments]: if dir contains no WAL segments.
//   - [ErrInvalidSegmentRange]: if wantFrom > wantTo or the requested range does not overlap
//     the existing segments.
func SearchRootHashForward(
	lg zerolog.Logger,
	expectedHash ledger.RootHash,
	dir string,
	wantFrom, wantTo int,
) (int, int64, error) {
	return searchRootHash(lg, expectedHash, dir, wantFrom, wantTo, false)
}

// SearchRootHashBackward scans WAL segments in backward order (last to first) and returns
// the segment index and byte offset of the last occurrence of expectedHash within the
// latest segment that contains it.
//
// Iterating backwards is more efficient when the target root hash is near the end of
// the WAL — for example when locating the state commitment of the last sealed block.
// Records within each individual segment are still read in forward order; only the
// segment iteration order is reversed.
//
// wantFrom and wantTo are inclusive bounds on the segment range to search; pass 0 and
// [math.MaxInt32] to search all available segments.
//
// Expected error returns during normal operation:
//   - [ErrRootHashNotFound]: if expectedHash does not appear in the searched segment range.
//   - [ErrNoWALSegments]: if dir contains no WAL segments.
//   - [ErrInvalidSegmentRange]: if wantFrom > wantTo or the requested range does not overlap
//     the existing segments.
func SearchRootHashBackward(
	lg zerolog.Logger,
	expectedHash ledger.RootHash,
	dir string,
	wantFrom, wantTo int,
) (int, int64, error) {
	return searchRootHash(lg, expectedHash, dir, wantFrom, wantTo, true)
}

// searchRootHash implements both forward and backward WAL segment scanning.
// When backward is true segments are iterated last-to-first; records within each
// segment are always read in forward order.
//
// Expected error returns during normal operation:
//   - [ErrRootHashNotFound]: if expectedHash does not appear in the searched segment range.
//   - [ErrNoWALSegments]: if dir contains no WAL segments.
//   - [ErrInvalidSegmentRange]: if wantFrom > wantTo or the requested range does not overlap
//     the existing segments.
func searchRootHash(
	lg zerolog.Logger,
	expectedHash ledger.RootHash,
	dir string,
	wantFrom, wantTo int,
	backward bool,
) (int, int64, error) {
	if wantFrom > wantTo {
		return 0, 0, fmt.Errorf("wantFrom %d is greater than wantTo %d: %w", wantFrom, wantTo, ErrInvalidSegmentRange)
	}

	from, to, err := prometheusWAL.Segments(dir)
	if err != nil {
		return 0, 0, irrecoverable.NewExceptionf("cannot get segments: %w", err)
	}

	if from < 0 {
		return 0, 0, fmt.Errorf("no segments found in %s: %w", dir, ErrNoWALSegments)
	}

	if wantFrom > to {
		return 0, 0, fmt.Errorf("from segment %d is greater than the last segment %d: %w", wantFrom, to, ErrInvalidSegmentRange)
	}

	if wantTo < from {
		return 0, 0, fmt.Errorf("to segment %d is less than the first segment %d: %w", wantTo, from, ErrInvalidSegmentRange)
	}

	if wantFrom > from {
		from = wantFrom
	}

	if wantTo < to {
		to = wantTo
	}

	lg.Info().
		Str("dir", dir).
		Int("from", from).
		Int("to", to).
		Bool("backward", backward).
		Msgf("searching for trie root hash %v in segments [%d,%d]", expectedHash, from, to)

	if !backward {
		return searchForward(lg, expectedHash, dir, from, to)
	}
	return searchBackward(lg, expectedHash, dir, from, to)
}

// searchForward scans segments from first to last, returning the position of the
// first occurrence of expectedHash.
//
// Expected error returns during normal operation:
//   - [ErrRootHashNotFound]: if expectedHash does not appear in the searched segment range.
func searchForward(
	lg zerolog.Logger,
	expectedHash ledger.RootHash,
	dir string,
	from, to int,
) (int, int64, error) {
	sr, err := prometheusWAL.NewSegmentsRangeReader(lg, prometheusWAL.SegmentRange{
		Dir:   dir,
		First: from,
		Last:  to,
	})
	if err != nil {
		return 0, 0, irrecoverable.NewExceptionf("cannot create WAL segments reader: %w", err)
	}
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)

	var foundSeg int
	var foundOffset int64
	found := false

	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			return 0, 0, irrecoverable.NewExceptionf("cannot decode LedgerWAL record at segment %d offset %d: %w",
				reader.Segment(), reader.Offset(), err)
		}

		if !found && operation == wal.WALUpdate && update.RootHash.Equals(expectedHash) {
			foundSeg = reader.Segment()
			foundOffset = reader.Offset()
			found = true
		}
	}

	if err := reader.Err(); err != nil {
		return 0, 0, irrecoverable.NewExceptionf("cannot read LedgerWAL: %w", err)
	}

	if !found {
		return 0, 0, fmt.Errorf("root hash not found in segments [%d,%d]: %w", from, to, ErrRootHashNotFound)
	}

	return foundSeg, foundOffset, nil
}

// searchBackward iterates segments from last to first. Within each segment records are
// read in forward order; the LAST matching record position is tracked so that
// findRootHashAndCreateTrimmed can trim the WAL cleanly at that point.
// The function returns the position of the last occurrence of expectedHash within the
// latest segment that contains it.
//
// Expected error returns during normal operation:
//   - [ErrRootHashNotFound]: if expectedHash does not appear in the searched segment range.
func searchBackward(
	lg zerolog.Logger,
	expectedHash ledger.RootHash,
	dir string,
	from, to int,
) (int, int64, error) {
	for seg := to; seg >= from; seg-- {
		foundSeg, foundOffset, err := scanSegmentForLastOccurrence(lg, expectedHash, dir, seg)
		if err != nil {
			return 0, 0, err
		}
		if foundSeg >= 0 {
			return foundSeg, foundOffset, nil
		}
	}

	return 0, 0, fmt.Errorf("root hash not found in segments [%d,%d]: %w", from, to, ErrRootHashNotFound)
}

// scanSegmentForLastOccurrence reads all records in the given segment forward and returns
// the position of the last occurrence of expectedHash. Returns (-1, 0, nil) if the hash
// is not present in the segment.
//
// No error returns are expected during normal operation.
func scanSegmentForLastOccurrence(
	lg zerolog.Logger,
	expectedHash ledger.RootHash,
	dir string,
	seg int,
) (int, int64, error) {
	segment, err := prometheusWAL.OpenReadSegment(prometheusWAL.SegmentName(dir, seg))
	if err != nil {
		return 0, 0, irrecoverable.NewExceptionf("cannot open segment %d: %w", seg, err)
	}

	sr := prometheusWAL.NewSegmentBufReader(lg, segment)
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)

	foundSeg := -1
	var foundOffset int64

	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			return 0, 0, irrecoverable.NewExceptionf("cannot decode LedgerWAL record in segment %d at offset %d: %w",
				seg, reader.Offset(), err)
		}

		if operation == wal.WALUpdate && update.RootHash.Equals(expectedHash) {
			foundSeg = reader.Segment()
			foundOffset = reader.Offset()
		}
	}

	if err := reader.Err(); err != nil {
		return 0, 0, irrecoverable.NewExceptionf("cannot read LedgerWAL in segment %d: %w", seg, err)
	}

	return foundSeg, foundOffset, nil
}

// DefaultWALFrom is the default lower bound for WAL segment search (inclusive).
const DefaultWALFrom = 0

// DefaultWALTo is the default upper bound for WAL segment search (inclusive).
const DefaultWALTo = math.MaxInt32
