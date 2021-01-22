package wal

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	prometheusWAL "github.com/prometheus/tsdb/wal"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
)

const SegmentSize = 32 * 1024 * 1024

type LedgerWAL struct {
	wal            *prometheusWAL.WAL
	paused         bool
	forestCapacity int
	pathByteSize   int
}

// TODO use real logger and metrics, but that would require passing them to Trie storage
func NewWAL(logger log.Logger, reg prometheus.Registerer, dir string, forestCapacity int, pathByteSize int, segmentSize int) (*LedgerWAL, error) {
	w, err := prometheusWAL.NewSize(logger, reg, dir, segmentSize)
	if err != nil {
		return nil, err
	}
	return &LedgerWAL{
		wal:            w,
		paused:         false,
		forestCapacity: forestCapacity,
		pathByteSize:   pathByteSize,
	}, nil
}

func (w *LedgerWAL) PauseRecord() {
	w.paused = true
}

func (w *LedgerWAL) UnpauseRecord() {
	w.paused = false
}

func (w *LedgerWAL) RecordUpdate(update *ledger.TrieUpdate) error {
	if w.paused {
		return nil
	}

	bytes := EncodeUpdate(update)

	err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording update in LedgerWAL: %w", err)
	}
	return nil
}

func (w *LedgerWAL) RecordDelete(rootHash ledger.RootHash) error {
	if w.paused {
		return nil
	}

	bytes := EncodeDelete(rootHash)

	err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording delete in LedgerWAL: %w", err)
	}
	return nil
}

func (w *LedgerWAL) ReplayOnForest(forest *mtrie.Forest) error {
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
			_, err := forest.Update(update)
			return err
		},
		func(rootHash ledger.RootHash) error {
			forest.RemoveTrie(rootHash)
			return nil
		},
	)
}

func (w *LedgerWAL) Replay(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(ledger.RootHash) error,
) error {
	from, to, err := w.wal.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, true)
}

func (w *LedgerWAL) ReplayLogsOnly(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
) error {
	from, to, err := w.wal.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, false)
}

func (w *LedgerWAL) replay(
	from, to int,
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
	useCheckpoints bool,
) error {

	if to < from {
		return fmt.Errorf("end of range cannot be smaller than beginning")
	}

	loadedCheckpoint := false
	startSegment := from

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return fmt.Errorf("cannot create checkpointer: %w", err)
	}

	if useCheckpoints {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		if err != nil {
			return fmt.Errorf("cannot get latest checkpoint: %w", err)
		}

		if latestCheckpoint != -1 && latestCheckpoint+1 >= from { //+1 to account for connected checkpoint and segments
			forestSequencing, err := checkpointer.LoadCheckpoint(latestCheckpoint)
			if err != nil {
				return fmt.Errorf("cannot load checkpoint %d: %w", latestCheckpoint, err)
			}
			err = checkpointFn(forestSequencing)
			if err != nil {
				return fmt.Errorf("error while handling checkpoint: %w", err)
			}
			loadedCheckpoint = true
		}

		if loadedCheckpoint && to == latestCheckpoint {
			return nil
		}

		if loadedCheckpoint {
			startSegment = latestCheckpoint + 1
		}
	}

	if !loadedCheckpoint && startSegment == 0 {
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
	return nil
}

// NewCheckpointer returns a Checkpointer for this WAL
func (w *LedgerWAL) NewCheckpointer() (*Checkpointer, error) {
	return NewCheckpointer(w, w.pathByteSize, w.forestCapacity), nil
}

func (w *LedgerWAL) Close() error {
	return w.wal.Close()
}
