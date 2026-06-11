package wal

import (
	"fmt"
	"sort"

	"github.com/hashicorp/go-multierror"
	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/module"
	utilsio "github.com/onflow/flow-go/utils/io"
)

const SegmentSize = 32 * 1024 * 1024 // 32 MB

type DiskWAL struct {
	wal            *prometheusWAL.WAL
	paused         bool
	forestCapacity int
	pathByteSize   int
	log            zerolog.Logger
	dir            string
	fileLock       *utilsio.FileLock
}

// TODO use real logger and metrics, but that would require passing them to Trie storage
func NewDiskWAL(logger zerolog.Logger, reg prometheus.Registerer, metrics module.WALMetrics, dir string, forestCapacity int, pathByteSize int, segmentSize int) (*DiskWAL, error) {
	// Acquire exclusive file lock to ensure only one process can write to this WAL directory
	fileLock, err := utilsio.NewFileLock(dir)
	if err != nil {
		panic(fmt.Sprintf("failed to create file lock for WAL directory %s: %v", dir, err))
	}
	if err := fileLock.Lock(); err != nil {
		// The Lock() method returns a complete error message that distinguishes between
		// permission denied and lock conflicts. This is a fatal error - the process should crash.
		panic(err.Error())
	}

	w, err := prometheusWAL.NewSize(logger, reg, dir, segmentSize, false)
	if err != nil {
		// Release the lock if WAL creation fails
		err = fmt.Errorf("could not create disk wal from dir %v, segmentSize %v: %w", dir, segmentSize, err)
		if unlockErr := fileLock.Unlock(); unlockErr != nil {
			err = multierror.Append(err, fmt.Errorf("failed to release file lock: %w", unlockErr))
		}
		return nil, err
	}

	log := logger.With().Str("ledger_mod", "diskwal").Logger()
	log.Info().Str("lock_path", fileLock.Path()).Msg("acquired exclusive lock on WAL directory")

	return &DiskWAL{
		wal:            w,
		paused:         false,
		forestCapacity: forestCapacity,
		pathByteSize:   pathByteSize,
		log:            log,
		dir:            dir,
		fileLock:       fileLock,
	}, nil
}

func (w *DiskWAL) PauseRecord() {
	w.paused = true
}

func (w *DiskWAL) UnpauseRecord() {
	w.paused = false
}

// RecordUpdate writes the trie update to the write ahead log on disk.
// if write ahead logging is not paused, it returns the file num (write ahead log) that the trie update was written to.
// if write ahead logging is enabled, the second returned value is false, otherwise it's true, meaning WAL is disabled.
func (w *DiskWAL) RecordUpdate(update *ledger.TrieUpdate) (segmentNum int, skipped bool, err error) {
	if w.paused {
		return 0, true, nil
	}

	bytes := EncodeUpdate(update)

	locations, err := w.wal.Log(bytes)

	if err != nil {
		return 0, false, fmt.Errorf("error while recording update in LedgerWAL: %w", err)
	}
	if len(locations) != 1 {
		return 0, false, fmt.Errorf("error while recording update in LedgerWAL: got %d location, expect 1 location", len(locations))
	}

	return locations[0].Segment, false, nil
}

func (w *DiskWAL) RecordDelete(rootHash ledger.RootHash) error {
	if w.paused {
		return nil
	}

	bytes := EncodeDelete(rootHash)

	_, err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording delete in LedgerWAL: %w", err)
	}
	return nil
}

func (w *DiskWAL) ReplayOnForest(forest *mtrie.Forest) error {
	return w.Replay(
		func(tries []*trie.MTrie) error {
			err := forest.AddTries(tries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(update *ledger.TrieUpdate) error {
			_, err := forest.Update(update)
			return err
		},
		func(rootHash ledger.RootHash) error {
			return nil
		},
	)
}

// ReplayOnPayloadlessForest reconstructs in-memory payloadless state by loading
// the latest V7 (payloadless) checkpoint from the WAL directory onto `forest`,
// then replaying every WAL segment newer than that checkpoint.
//
// This is the payloadless analog of [DiskWAL.ReplayOnForest]: it hides
// checkpoint selection, checkpoint loading, and segment replay behind a single
// call so the ledger constructor stays uniform across V6 and V7. Like the V6
// path, it tries the newest V7 checkpoint first and falls back to older ones if
// a checkpoint file fails to load. When no V7 checkpoint exists, it replays all
// segments onto the (presumably empty) `forest`.
//
// When no numbered V7 checkpoint is available it falls back to a V7 root
// checkpoint (converted from the V6 root.checkpoint during bootstrap), mirroring
// the V6 root-checkpoint fallback in [DiskWAL.replay]. With no V7 checkpoint of
// either kind it replays all segments onto the (presumably empty) `forest`.
//
// No error returns are expected during normal operation.
func (w *DiskWAL) ReplayOnPayloadlessForest(forest *payloadless.Forest) error {
	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return fmt.Errorf("cannot create checkpointer: %w", err)
	}

	checkpoints, err := checkpointer.CheckpointsV7()
	if err != nil {
		return fmt.Errorf("cannot list V7 checkpoints: %w", err)
	}

	// Try the newest V7 checkpoint first, falling back to older ones if a file
	// fails to load. This mirrors the V6 checkpoint selection in [DiskWAL.replay].
	loadedCheckpoint := -1
	for i := len(checkpoints) - 1; i >= 0; i-- {
		num := checkpoints[i]
		name := NumberToFilenameV7(num)
		tries, err := OpenAndReadCheckpointV7(checkpointer.Dir(), name, w.log)
		if err != nil {
			w.log.Warn().Int("checkpoint", num).Err(err).
				Msg("V7 checkpoint loading failed; falling back to older checkpoint")
			continue
		}
		if err := forest.AddTries(tries); err != nil {
			return fmt.Errorf("failed to seed payloadless forest from V7 checkpoint %s: %w", name, err)
		}
		w.log.Info().Int("checkpoint", num).Int("trie_count", len(tries)).
			Msg("payloadless forest seeded from V7 checkpoint")
		loadedCheckpoint = num
		break
	}

	// No numbered V7 checkpoint loaded: fall back to the V7 root checkpoint, if
	// present. This is the payloadless analog of the root-checkpoint branch in
	// [DiskWAL.replay]; like that branch it does not advance the replay start, so
	// all segments are replayed on top of the root state.
	if loadedCheckpoint == -1 {
		hasV7Root, err := checkpointer.HasRootCheckpointV7()
		if err != nil {
			return fmt.Errorf("cannot check for V7 root checkpoint: %w", err)
		}
		if hasV7Root {
			tries, err := checkpointer.LoadRootCheckpointV7()
			if err != nil {
				return fmt.Errorf("failed to load V7 root checkpoint: %w", err)
			}
			if err := forest.AddTries(tries); err != nil {
				return fmt.Errorf("failed to seed payloadless forest from V7 root checkpoint: %w", err)
			}
			w.log.Info().Int("trie_count", len(tries)).
				Msg("payloadless forest seeded from V7 root checkpoint")
		}
	}

	return w.replaySegmentsForPayloadlessForest(forest, loadedCheckpoint)
}

// replaySegmentsForPayloadlessForest replays WAL segments onto a payloadless
// forest, skipping any segments that are already covered by the checkpoint that
// [DiskWAL.ReplayOnPayloadlessForest] already loaded into `forest`. It is the
// segment-replay half of that method.
//
// `afterCheckpointNum` is the number of the loaded checkpoint; segments through
// that number are skipped. Pass -1 (or any value < firstSegment) to replay all
// segments — used when no checkpoint, or only a V7 root checkpoint, was loaded.
//
// Unlike [DiskWAL.ReplayOnForest] this does NOT call the V6 checkpoint callback —
// V6 checkpoints are not directly loadable into a payloadless forest. Delete
// records are ignored (the WAL has no segment-level concept of trie deletion
// that needs to be reflected in the payloadless forest).
//
// No error returns are expected during normal operation.
func (w *DiskWAL) replaySegmentsForPayloadlessForest(
	forest *payloadless.Forest,
	afterCheckpointNum int,
) error {
	firstSeg, lastSeg, err := w.Segments()
	if err != nil {
		return fmt.Errorf("could not find segments: %w", err)
	}
	from := firstSeg
	if afterCheckpointNum >= from {
		from = afterCheckpointNum + 1
	}
	if from > lastSeg {
		// V7 checkpoint already covers everything on disk.
		return nil
	}
	err = w.replay(from, lastSeg,
		func(tries []*trie.MTrie) error { return nil }, // unused when useCheckpoints=false
		func(update *ledger.TrieUpdate) error {
			_, err := forest.Update(update)
			return err
		},
		func(rootHash ledger.RootHash) error { return nil },
		false, // useCheckpoints
	)
	if err != nil {
		return fmt.Errorf("could not replay WAL segments [%v:%v] for payloadless forest: %w", from, lastSeg, err)
	}
	return nil
}

func (w *DiskWAL) Segments() (first, last int, err error) {
	return prometheusWAL.Segments(w.wal.Dir())
}

func (w *DiskWAL) Replay(
	checkpointFn func(tries []*trie.MTrie) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(ledger.RootHash) error,
) error {
	from, to, err := w.Segments()
	if err != nil {
		return fmt.Errorf("could not find segments: %w", err)
	}
	err = w.replay(from, to, checkpointFn, updateFn, deleteFn, true)
	if err != nil {
		return fmt.Errorf("could not replay segments [%v:%v]: %w", from, to, err)
	}
	return nil
}

func (w *DiskWAL) ReplayLogsOnly(
	checkpointFn func(tries []*trie.MTrie) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
) error {
	from, to, err := w.Segments()
	if err != nil {
		return fmt.Errorf("could not find segments: %w", err)
	}
	err = w.replay(from, to, checkpointFn, updateFn, deleteFn, false)
	if err != nil {
		return fmt.Errorf("could not replay WAL only for segments [%v:%v]: %w", from, to, err)
	}
	return nil
}

func (w *DiskWAL) replay(
	from, to int,
	checkpointFn func(tries []*trie.MTrie) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
	useCheckpoints bool,
) error {

	w.log.Info().Msgf("loading checkpoint with WAL from %d to %d, useCheckpoints %v", from, to, useCheckpoints)

	if to < from {
		return fmt.Errorf("end of range cannot be smaller than beginning")
	}

	loadedCheckpoint := -1
	startSegment := from
	checkpointLoaded := false

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return fmt.Errorf("cannot create checkpointer: %w", err)
	}

	if useCheckpoints {
		// Only consider V6 checkpoints here: this replay path loads checkpoints via
		// LoadCheckpointV6 (full mtrie). V7 (payloadless) files may live in the same
		// directory but are not loadable here, so including them would only cause
		// failed load attempts and misleading warnings before falling back to a V6
		// checkpoint. This mirrors the V6-only enumeration used by the checkpoint
		// scheduling logic (see Checkpointer.listV6Checkpoints).
		allCheckpoints, err := checkpointer.CheckpointsV6()
		if err != nil {
			return fmt.Errorf("cannot get list of checkpoints: %w", err)
		}

		var availableCheckpoints []int

		// if there are no checkpoints already, don't bother
		if len(allCheckpoints) > 0 {
			// from-1 to account for checkpoints connected to segments, ie. checkpoint 8 if replaying segments 9-12
			availableCheckpoints = getPossibleCheckpoints(allCheckpoints, from-1, to)
		}

		w.log.Info().Ints("checkpoints", availableCheckpoints).Msg("available checkpoints")

		for len(availableCheckpoints) > 0 {
			// as long as there are checkpoints to try, we always try with the last checkpoint file, since
			// it allows us to load less segments.
			latestCheckpoint := availableCheckpoints[len(availableCheckpoints)-1]

			w.log.Info().Int("checkpoint", latestCheckpoint).Msg("loading checkpoint")

			forestSequencing, err := checkpointer.LoadCheckpoint(latestCheckpoint)
			if err != nil {
				w.log.Warn().Int("checkpoint", latestCheckpoint).Err(err).
					Msg("checkpoint loading failed")

				availableCheckpoints = availableCheckpoints[:len(availableCheckpoints)-1]
				continue
			}

			if len(forestSequencing) == 0 {
				return fmt.Errorf("checkpoint loaded but has no trie")
			}

			firstTrie := forestSequencing[0].RootHash()
			lastTrie := forestSequencing[len(forestSequencing)-1].RootHash()
			w.log.Info().Int("checkpoint", latestCheckpoint).
				Hex("first_trie", firstTrie[:]).
				Hex("last_trie", lastTrie[:]).
				Msg("checkpoint loaded")

			err = checkpointFn(forestSequencing)
			if err != nil {
				return fmt.Errorf("error while handling checkpoint: %w", err)
			}
			loadedCheckpoint = latestCheckpoint
			checkpointLoaded = true
			break
		}

		if loadedCheckpoint != -1 && loadedCheckpoint == to {
			w.log.Info().Msgf("no checkpoint to load")
			return nil
		}

		if loadedCheckpoint >= 0 {
			startSegment = loadedCheckpoint + 1
		}

		w.log.Info().
			Int("start_segment", startSegment).
			Msg("starting replay from checkpoint segment")
	}

	if loadedCheckpoint == -1 && startSegment == 0 {
		hasRootCheckpoint, err := checkpointer.HasRootCheckpoint()
		if err != nil {
			return fmt.Errorf("cannot check root checkpoint existence: %w", err)
		}
		if hasRootCheckpoint {
			w.log.Info().Msgf("loading root checkpoint")

			flattenedForest, err := checkpointer.LoadRootCheckpoint()
			if err != nil {
				return fmt.Errorf("cannot load root checkpoint: %w", err)
			}
			err = checkpointFn(flattenedForest)
			if err != nil {
				return fmt.Errorf("error while handling root checkpoint: %w", err)
			}

			rootHash := flattenedForest[len(flattenedForest)-1].RootHash()
			w.log.Info().
				Hex("root_hash", rootHash[:]).
				Msg("root checkpoint loaded")
			checkpointLoaded = true
		} else {
			w.log.Info().Msgf("no root checkpoint was found")
		}
	}

	w.log.Info().
		Bool("checkpoint_loaded", checkpointLoaded).
		Int("loaded_checkpoint", loadedCheckpoint).
		Msgf("replaying segments from %d to %d", startSegment, to)

	sr, err := prometheusWAL.NewSegmentsRangeReader(w.log, prometheusWAL.SegmentRange{
		Dir:   w.wal.Dir(),
		First: startSegment,
		Last:  to,
	})
	if err != nil {
		return fmt.Errorf("cannot create segment reader: %w", err)
	}

	reader := prometheusWAL.NewReader(sr)

	defer sr.Close()

	for reader.Next() {
		record := reader.Record()
		operation, rootHash, update, err := Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode LedgerWAL record: %w", err)
		}

		switch operation {
		case WALUpdate:
			err = updateFn(update)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL update: %w", err)
			}
		case WALDelete:
			err = deleteFn(rootHash)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL deletion: %w", err)
			}
		}

		err = reader.Err()
		if err != nil {
			return fmt.Errorf("cannot read LedgerWAL: %w", err)
		}
	}

	w.log.Info().Msgf("finished loading checkpoint and replaying WAL from %d to %d", from, to)

	return nil
}

func getPossibleCheckpoints(allCheckpoints []int, from, to int) []int {
	// list of checkpoints is sorted
	indexFrom := sort.SearchInts(allCheckpoints, from)
	indexTo := sort.SearchInts(allCheckpoints, to)

	// all checkpoints are earlier, return last one
	if indexTo == len(allCheckpoints) {
		return allCheckpoints[indexFrom:indexTo]
	}

	// exact match
	if allCheckpoints[indexTo] == to {
		return allCheckpoints[indexFrom : indexTo+1]
	}

	// earliest checkpoint from list doesn't match, index 0 means no match at all
	if indexTo == 0 {
		return nil
	}

	return allCheckpoints[indexFrom:indexTo]
}

// NewCheckpointer returns a Checkpointer for this WAL
func (w *DiskWAL) NewCheckpointer() (*Checkpointer, error) {
	return NewCheckpointer(w, w.pathByteSize, w.forestCapacity), nil
}

func (w *DiskWAL) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done implements interface module.ReadyDoneAware
// it closes all the open write-ahead log files and releases the file lock.
func (w *DiskWAL) Done() <-chan struct{} {
	err := w.wal.Close()
	if err != nil {
		w.log.Err(err).Msg("error while closing WAL")
	}

	// Release the file lock
	if w.fileLock != nil {
		if err := w.fileLock.Unlock(); err != nil {
			w.log.Err(err).Msg("error while releasing file lock")
		} else {
			w.log.Info().Str("lock_path", w.fileLock.Path()).Msg("released exclusive lock on WAL directory")
		}
	}

	done := make(chan struct{})
	close(done)
	return done
}

type LedgerWAL interface {
	module.ReadyDoneAware

	NewCheckpointer() (*Checkpointer, error)
	PauseRecord()
	UnpauseRecord()
	RecordUpdate(update *ledger.TrieUpdate) (int, bool, error)
	RecordDelete(rootHash ledger.RootHash) error
	ReplayOnForest(forest *mtrie.Forest) error
	ReplayOnPayloadlessForest(forest *payloadless.Forest) error
	Segments() (first, last int, err error)
	Replay(
		checkpointFn func(tries []*trie.MTrie) error,
		updateFn func(update *ledger.TrieUpdate) error,
		deleteFn func(ledger.RootHash) error,
	) error
	ReplayLogsOnly(
		checkpointFn func(tries []*trie.MTrie) error,
		updateFn func(update *ledger.TrieUpdate) error,
		deleteFn func(rootHash ledger.RootHash) error,
	) error
}
