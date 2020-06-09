package wal

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	prometheusWAL "github.com/prometheus/tsdb/wal"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/sequencer"

	"github.com/dapperlabs/flow-go/model/flow"
)

type LedgerWAL struct {
	wal       *prometheusWAL.WAL
	cacheSize int
	maxHeight int
}

// TODO use real loger and metrics, but that would require passing them to Trie storage
func NewWAL(logger log.Logger, reg prometheus.Registerer, dir string, cacheSize int, maxHeight int) (*LedgerWAL, error) {
	w, err := prometheusWAL.NewSize(logger, reg, dir, 32*1024)
	if err != nil {
		return nil, err
	}
	return &LedgerWAL{
		wal:       w,
		cacheSize: cacheSize,
		maxHeight: maxHeight,
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

func (w *LedgerWAL) Replay(
	checkpointFn func(forestSequencing *sequencer.MForestSequencing) error,
	updateFn func(flow.StateCommitment, [][]byte, [][]byte) error,
	deleteFn func(flow.StateCommitment) error,
) error {
	from, to, err := w.wal.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn)
}

func (w *LedgerWAL) replay(
	from, to int,
	checkpointFn func(forestSequencing *sequencer.MForestSequencing) error,
	updateFn func(flow.StateCommitment, [][]byte, [][]byte) error,
	deleteFn func(flow.StateCommitment) error,
) error {

	if to < from {
		return fmt.Errorf("end of range cannot be smaller than beginning")
	}

	checkpointer, err := w.Checkpointer()
	if err != nil {
		return fmt.Errorf("cannot create checkpointer: %w", err)
	}

	latestCheckpoint, err := checkpointer.LatestCheckpoint()
	if err != nil {
		return fmt.Errorf("cannot get latest checkpoint: %w", err)
	}

	loadedCheckpoint := false

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

	startSegment := from
	if loadedCheckpoint {
		startSegment = latestCheckpoint + 1
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

func (w *LedgerWAL) Checkpointer() (*Checkpointer, error) {
	return NewCheckpointer(w, w.maxHeight, w.cacheSize), nil
}

func (w *LedgerWAL) Close() error {
	return w.wal.Close()
}
