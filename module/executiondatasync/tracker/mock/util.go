package mocktracker

import (
	"github.com/ipfs/go-cid"
	mock "github.com/stretchr/testify/mock"

	tracker "github.com/onflow/flow-go/module/executiondatasync/tracker"
)

func NewMockStorage() *Storage {
	trackerStorage := new(Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

	trackerStorage.On("SetFulfilledHeight", mock.Anything).Return(nil)

	return trackerStorage
}
