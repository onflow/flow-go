package wal

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	prometheusWAL "github.com/m4ksio/wal/wal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/io"
)

const SegmentSize = 32 * 1024 * 1024

var disable = false

type DiskWAL struct {
	wal            *prometheusWAL.WAL
	paused         bool
	forestCapacity int
	pathByteSize   int
	log            zerolog.Logger
	// disk size reading can be time consuming, so limit how often its read
	diskUpdateLimiter *time.Ticker
	metrics           module.WALMetrics
	dir               string
	disabled          bool
}

// TODO use real logger and metrics, but that would require passing them to Trie storage
func NewDiskWAL(logger zerolog.Logger, reg prometheus.Registerer, metrics module.WALMetrics, dir string, forestCapacity int, pathByteSize int, segmentSize int) (*DiskWAL, error) {
	w, err := prometheusWAL.NewSize(logger, reg, dir, segmentSize, false)
	if err != nil {
		return nil, err
	}
	return &DiskWAL{
		wal:               w,
		paused:            false,
		forestCapacity:    forestCapacity,
		pathByteSize:      pathByteSize,
		log:               logger,
		diskUpdateLimiter: time.NewTicker(5 * time.Second),
		metrics:           metrics,
		dir:               dir,
	}, nil
}

func (w *DiskWAL) PauseRecord() {
	w.paused = true
}

func (w *DiskWAL) UnpauseRecord() {
	w.paused = false
}

func (w *DiskWAL) RecordUpdate(update *ledger.TrieUpdate) error {
	if w.paused {
		return nil
	}

	bytes := EncodeUpdate(update)

	_, err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording update in LedgerWAL: %w", err)
	}

	select {
	case <-w.diskUpdateLimiter.C:
		diskSize, err := w.DiskSize()
		if err != nil {
			w.log.Warn().Err(err).Msg("error while checking forest disk size")
		} else {
			w.metrics.DiskSize(diskSize)
		}
	default: //don't block
	}

	return nil
}

// DiskSize returns the amount of disk space used by the storage (in bytes)
func (w *DiskWAL) DiskSize() (uint64, error) {
	return io.DirSize(w.dir)
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
		func(forestSequencing *flattener.FlattenedForest) error {
			rebuiltTries, err := flattener.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = forest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(update *ledger.TrieUpdate) error {

			if !w.disabled {
				_, err := forest.Update(update)
				if err != nil {
					fmt.Println(err.Error())
				}
			}
			fmt.Println(">>>>", update.RootHash.String())

			waitingHash, err := flow.HexStringToIdentifier("0f045d7f5e7ae523c08cea0159786eac043b5f16d1e731f32f2ef0875c23d96d")
			if err != nil {
				panic(err)
			}
			if bytes.Equal(waitingHash[:], update.RootHash[:]) {
				encodedUpdate := EncodeUpdate(update)
				err := ioutil.WriteFile("update.bin", encodedUpdate, 0644)
				if err != nil {
					panic(err)
				}
				fmt.Println("found the target trie, dumping as json")
				w.disabled = true
			}
			return nil
		},
		func(rootHash ledger.RootHash) error {
			if !w.disabled {
				forest.RemoveTrie(rootHash)
			}
			return nil
		},
	)
}

func (w *DiskWAL) Segments() (first, last int, err error) {
	return prometheusWAL.Segments(w.wal.Dir())
}

func (w *DiskWAL) Replay(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(ledger.RootHash) error,
) error {
	from, to, err := w.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, true)
}

func (w *DiskWAL) ReplayLogsOnly(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
) error {
	from, to, err := w.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, false)
}

func (w *DiskWAL) replay(
	from, to int,
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
	useCheckpoints bool,
) error {

	w.log.Debug().Msgf("replaying WAL from %d to %d", from, to)

	if to < from {
		return fmt.Errorf("end of range cannot be smaller than beginning")
	}

	loadedCheckpoint := -1
	startSegment := from

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return fmt.Errorf("cannot create checkpointer: %w", err)
	}

	if useCheckpoints {
		allCheckpoints, err := checkpointer.Checkpoints()
		if err != nil {
			return fmt.Errorf("cannot get list of checkpoints: %w", err)
		}

		var availableCheckpoints []int

		// if there are no checkpoints already, don't bother
		if len(allCheckpoints) > 0 {
			// from-1 to account for checkpoints connected to segments, ie. checkpoint 8 if replaying segments 9-12
			availableCheckpoints = getPossibleCheckpoints(allCheckpoints, from-1, to)
		}

		for len(availableCheckpoints) > 0 {
			// as long as there are checkpoints to try, we always try with the last checkpoint file, since
			// it allows us to load less segments.
			latestCheckpoint := availableCheckpoints[len(availableCheckpoints)-1]

			forestSequencing, err := checkpointer.LoadCheckpoint(latestCheckpoint)
			if err != nil {
				w.log.Warn().Int("checkpoint", latestCheckpoint).Err(err).
					Msg("checkpoint loading failed")

				availableCheckpoints = availableCheckpoints[:len(availableCheckpoints)-1]
				continue
			}
			w.log.Info().Int("checkpoint", latestCheckpoint).
				Msg("checkpoint loaded")
			err = checkpointFn(forestSequencing)
			if err != nil {
				return fmt.Errorf("error while handling checkpoint: %w", err)
			}
			loadedCheckpoint = latestCheckpoint
			break
		}

		if loadedCheckpoint != -1 && loadedCheckpoint == to {
			return nil
		}

		if loadedCheckpoint >= 0 {
			startSegment = loadedCheckpoint + 1
		}
	}

	if loadedCheckpoint == -1 && startSegment == 0 {
		hasRootCheckpoint, err := checkpointer.HasRootCheckpoint()
		if err != nil {
			return fmt.Errorf("cannot check root checkpoint existence: %w", err)
		}
		if hasRootCheckpoint {
			flattenedForest, err := checkpointer.LoadRootCheckpoint()
			if err != nil {
				return fmt.Errorf("cannot load root checkpoint: %w", err)
			}
			err = checkpointFn(flattenedForest)
			if err != nil {
				return fmt.Errorf("error while handling root checkpoint: %w", err)
			}
		}
	}

	w.log.Debug().Msgf("replying segments from %d to %d", startSegment, to)

	sr, err := prometheusWAL.NewSegmentsRangeReader(prometheusWAL.SegmentRange{
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

	w.log.Debug().Msgf("finished replaying WAL from %d to %d", from, to)

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
// it closes all the open write-ahead log files.
func (w *DiskWAL) Done() <-chan struct{} {
	err := w.wal.Close()
	if err != nil {
		w.log.Err(err).Msg("error while closing WAL")
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
	RecordUpdate(update *ledger.TrieUpdate) error
	RecordDelete(rootHash ledger.RootHash) error
	ReplayOnForest(forest *mtrie.Forest) error
	Segments() (first, last int, err error)
	Replay(
		checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
		updateFn func(update *ledger.TrieUpdate) error,
		deleteFn func(ledger.RootHash) error,
	) error
	ReplayLogsOnly(
		checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
		updateFn func(update *ledger.TrieUpdate) error,
		deleteFn func(rootHash ledger.RootHash) error,
	) error
}
