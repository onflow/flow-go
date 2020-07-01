package wal

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	prometheusWAL "github.com/prometheus/tsdb/wal"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/flattener"

	"github.com/dapperlabs/flow-go/model/flow"
)

type LedgerWAL struct {
	wal            *prometheusWAL.WAL
	forestCapacity int
	keyByteSize    int
}

// TODO use real logger and metrics, but that would require passing them to Trie storage
func NewWAL(logger log.Logger, reg prometheus.Registerer, dir string, forestCapacity int, keyByteSize int) (*LedgerWAL, error) {
	w, err := prometheusWAL.NewSize(logger, reg, dir, 32*1024)
	if err != nil {
		return nil, err
	}
	return &LedgerWAL{
		wal:            w,
		forestCapacity: forestCapacity,
		keyByteSize:    keyByteSize,
	}, nil
}

func (w *LedgerWAL) RecordUpdate(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
	bytes := EncodeUpdate(stateCommitment, keys, values)

	err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording update in LedgerWAL: %w", err)
	}
	return nil
}

func (w *LedgerWAL) RecordDelete(stateCommitment flow.StateCommitment) error {
	bytes := EncodeDelete(stateCommitment)

	err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording delete in LedgerWAL: %w", err)
	}
	return nil
}

func (w *LedgerWAL) ReplayOnMForest(mForest *mtrie.MForest) error {
	return w.Replay(
		func(forestSequencing *flattener.FlattenedForest) error {
			rebuiltTries, err := flattener.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = mForest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			_, err := mForest.Update(stateCommitment, keys, values)
			// _, err := trie.UpdateRegisters(keys, values, stateCommitment)
			return err
		},
		func(stateCommitment flow.StateCommitment) error {
			mForest.RemoveTrie(stateCommitment)
			return nil
		},
	)
}

func (w *LedgerWAL) Replay(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(flow.StateCommitment, [][]byte, [][]byte) error,
	deleteFn func(flow.StateCommitment) error,
) error {
	from, to, err := w.wal.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, true)
}

func (w *LedgerWAL) ReplayLogsOnly(
	updateFn func(flow.StateCommitment, [][]byte, [][]byte) error,
	deleteFn func(flow.StateCommitment) error,
) error {
	from, to, err := w.wal.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, nil, updateFn, deleteFn, false)
}

func (w *LedgerWAL) replay(
	from, to int,
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(flow.StateCommitment, [][]byte, [][]byte) error,
	deleteFn func(flow.StateCommitment) error,
	useCheckpoints bool,
) error {

	if to < from {
		return fmt.Errorf("end of range cannot be smaller than beginning")
	}

	loadedCheckpoint := false
	startSegment := from

	if useCheckpoints {

		checkpointer, err := w.NewCheckpointer()
		if err != nil {
			return fmt.Errorf("cannot create checkpointer: %w", err)
		}

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
		operation, commitment, keys, values, err := Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode LedgerWAL record: %w", err)
		}

		switch operation {
		case WALUpdate:
			err = updateFn(commitment, keys, values)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL update: %w", err)
			}
		case WALDelete:
			err = deleteFn(commitment)
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
	return NewCheckpointer(w, w.keyByteSize, w.forestCapacity), nil
}

func (w *LedgerWAL) Close() error {
	return w.wal.Close()
}
