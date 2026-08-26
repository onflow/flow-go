package common

import (
	"errors"
	"fmt"
	"math"
	"os"

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

// TrimWALSegmentToHash reads the given WAL segment forward, copies every record up to
// and including the occurrence of targetHash at the selected offset into a new segment
// file inside outputDir, and returns the path of the newly created segment file.
//
// The selected offset must be the offset of a WALUpdate record whose root hash equals
// targetHash. Records that precede that offset are copied; the matching record is copied
// and the function returns immediately after it. Any earlier occurrence of targetHash is
// logged as a warning and is not treated as the stopping point.
//
// The new segment file is always named "00000000" (segment 0) inside outputDir.
// The caller is responsible for moving it to the correct destination after this call.
//
// No error returns are expected during normal operation.
func TrimWALSegmentToHash(
	lg zerolog.Logger,
	dir string,
	segment int,
	offset int64,
	targetHash ledger.RootHash,
	outputDir string,
) (string, error) {
	newSegmentFile := prometheusWAL.SegmentName(outputDir, 0)

	writer, err := prometheusWAL.NewSize(lg, nil, outputDir, wal.SegmentSize, false)
	if err != nil {
		return "", fmt.Errorf("cannot create WAL writer in %s: %w", outputDir, err)
	}
	defer writer.Close()

	segmentFile := prometheusWAL.SegmentName(dir, segment)
	sr, err := prometheusWAL.OpenReadSegment(segmentFile)
	if err != nil {
		return "", irrecoverable.NewExceptionf("cannot open segment %s: %w", segmentFile, err)
	}
	defer sr.Close()

	reader := prometheusWAL.NewReader(prometheusWAL.NewSegmentBufReader(lg, sr))
	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			return "", irrecoverable.NewExceptionf("cannot decode WAL record in segment %d at offset %d: %w", segment, reader.Offset(), err)
		}

		if operation != wal.WALUpdate {
			continue
		}

		if _, err = writer.Log(wal.EncodeUpdate(update)); err != nil {
			return "", fmt.Errorf("cannot write WAL record: %w", err)
		}

		if update.RootHash.Equals(targetHash) {
			if reader.Offset() < offset {
				lg.Warn().Msgf("expected trie root hash %v found at offset %d before selected offset %d, continuing", targetHash, reader.Offset(), offset)
			} else {
				lg.Info().Msgf("found expected trie root hash %v at offset %d, finish writing", targetHash, reader.Offset())
				return newSegmentFile, nil
			}
		}
	}

	if err := reader.Err(); err != nil {
		return "", irrecoverable.NewExceptionf("cannot read WAL in segment %d: %w", segment, err)
	}

	return "", fmt.Errorf("target hash not found in segment %d", segment)
}

// BackupAndReplaceWALSegment moves all WAL segments from segment index segment through
// the last segment from walDir into backupDir (preserving their names), then moves
// newSegmentFile into walDir as the replacement for segment.
//
// After this call walDir contains exactly the segments that preceded segment, plus the
// newly trimmed segment; all later segments live in backupDir.
//
// No error returns are expected during normal operation.
func BackupAndReplaceWALSegment(
	segment int,
	walDir, backupDir string,
	newSegmentFile string,
) error {
	first, last, err := prometheusWAL.Segments(walDir)
	if err != nil {
		return fmt.Errorf("cannot enumerate WAL segments: %w", err)
	}

	if segment < first {
		return fmt.Errorf("segment %d is before the first segment %d", segment, first)
	}

	for i := segment; i <= last; i++ {
		src := prometheusWAL.SegmentName(walDir, i)
		dst := prometheusWAL.SegmentName(backupDir, i)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("cannot move segment %d to backup: %w", i, err)
		}
	}

	dst := prometheusWAL.SegmentName(walDir, segment)
	if err := os.Rename(newSegmentFile, dst); err != nil {
		return fmt.Errorf("cannot replace segment %d: %w", segment, err)
	}

	return nil
}
