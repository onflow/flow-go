package util

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

type NoopExecutionDataTracker struct{}

var _ storage.ExecutionDataTracker = (*NoopExecutionDataTracker)(nil)

func (s *NoopExecutionDataTracker) Update(update storage.UpdateFn) error {
	return update(func(blockHeight uint64, cids ...cid.Cid) error {
		return nil
	})
}

func (s *NoopExecutionDataTracker) GetFulfilledHeight() (uint64, error) {
	return 0, nil
}

func (s *NoopExecutionDataTracker) SetFulfilledHeight(uint64) error {
	return nil
}

func (s *NoopExecutionDataTracker) GetPrunedHeight() (uint64, error) {
	return 0, nil
}

func (s *NoopExecutionDataTracker) PruneUpToHeight(height uint64) error {
	return nil
}
