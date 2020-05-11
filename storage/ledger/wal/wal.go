package wal

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	prometheusWAL "github.com/prometheus/tsdb/wal"

	"github.com/dapperlabs/flow-go/model/flow"
)

type WAL struct {
	w *prometheusWAL.WAL
}

func NewWAL(logger log.Logger, reg prometheus.Registerer, dir string) (*WAL, error) {
	w, err := prometheusWAL.New(logger, reg, dir)
	if err != nil {
		return nil, err
	}
	return &WAL{
		w: w,
	}, nil
}

func (w *WAL) RecordUpdate(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
	bytes := EncodeUpdate(stateCommitment, keys, values)

	err := w.w.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording update in WAL: %w", err)
	}
	return nil
}

func (w *WAL) RecordDelete(stateCommitment flow.StateCommitment) error {
	bytes := EncodeDelete(stateCommitment)

	err := w.w.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording delete in WAL: %w", err)
	}
	return nil
}

func (w *WAL) Close() error {
	return w.w.Close()
}
