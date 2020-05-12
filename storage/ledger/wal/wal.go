package wal

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	prometheusWAL "github.com/prometheus/tsdb/wal"

	"github.com/dapperlabs/flow-go/model/flow"
)

type WAL struct {
	wal *prometheusWAL.WAL
}

// TODO use real loger and metrics, but that would require passing them to Trie storage
func NewWAL(logger log.Logger, reg prometheus.Registerer, dir string) (*WAL, error) {
	w, err := prometheusWAL.New(logger, reg, dir)
	if err != nil {
		return nil, err
	}
	return &WAL{
		wal: w,
	}, nil
}

func (w *WAL) RecordUpdate(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
	bytes := EncodeUpdate(stateCommitment, keys, values)

	err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording update in WAL: %w", err)
	}
	return nil
}

func (w *WAL) RecordDelete(stateCommitment flow.StateCommitment) error {
	bytes := EncodeDelete(stateCommitment)

	err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording delete in WAL: %w", err)
	}
	return nil
}

func (w *WAL) Reply(
	updateFn func(flow.StateCommitment, [][]byte, [][]byte) error,
	deleteFn func(flow.StateCommitment) error,
) error {

	sr, err := prometheusWAL.NewSegmentsReader(w.wal.Dir())
	if err != nil {
		return fmt.Errorf("cannot create segment reader: %w", err)
	}

	reader := prometheusWAL.NewReader(sr)

	for reader.Next() {
		record := reader.Record()
		operation, commitment, keys, values, err := Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode WAL record: %w", err)
		}

		switch operation {
		case WALUpdate:
			err = updateFn(commitment, keys, values)
			if err != nil {
				return fmt.Errorf("error while processing WAL update: %w", err)
			}
		case WALDelete:
			err = deleteFn(commitment)
			if err != nil {
				return fmt.Errorf("error while processing WAL deletion: %w", err)
			}
		}

		err = reader.Err()
		if err != nil {
			return fmt.Errorf("cannot read WAL: %w", err)
		}
	}
	return nil
}

func (w *WAL) Close() error {
	return w.wal.Close()
}
