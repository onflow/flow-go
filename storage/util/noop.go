package util

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

type NoopStorage struct{}

var _ storage.ExecutionDataTracker = (*NoopStorage)(nil)

func (s *NoopStorage) Update(update storage.UpdateFn) error {
	return update(func(blockHeight uint64, cids ...cid.Cid) error {
		return nil
	})
}

func (s *NoopStorage) GetFulfilledHeight() (uint64, error) {
	return 0, nil
}

func (s *NoopStorage) SetFulfilledHeight(uint64) error {
	return nil
}

func (s *NoopStorage) GetPrunedHeight() (uint64, error) {
	return 0, nil
}

func (s *NoopStorage) PruneUpToHeight(height uint64) error {
	return nil
}
