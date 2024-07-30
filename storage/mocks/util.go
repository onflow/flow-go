package mocks

import (
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
)

func NewMockStorage() *storagemock.ExecutionDataTracker {
	trackerStorage := new(storagemock.ExecutionDataTracker)
	trackerStorage.On("Update", mock.Anything).Return(func(fn storage.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

	trackerStorage.On("SetFulfilledHeight", mock.Anything).Return(nil)

	return trackerStorage
}
