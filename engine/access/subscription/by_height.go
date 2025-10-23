package subscription

import (
	"context"
	"fmt"
)

// GetDataByHeightFunc is a callback used by subscriptions to retrieve data for a given height.
// Expected errors:
// - storage.ErrNotFound
// - execution_data.BlobNotFoundError
// All other errors are considered exceptions
type GetDataByHeightFunc func(ctx context.Context, height uint64) (interface{}, error)

// HeightBasedSubscription is a subscription that retrieves data sequentially by block height
type HeightBasedSubscription struct {
	*SubscriptionImpl
	nextHeight uint64
	getData    GetDataByHeightFunc
}

var _ Subscription = (*HeightBasedSubscription)(nil)
var _ Streamable = (*HeightBasedSubscription)(nil)

func NewHeightBasedSubscription(bufferSize int, firstHeight uint64, getData GetDataByHeightFunc) *HeightBasedSubscription {
	return &HeightBasedSubscription{
		SubscriptionImpl: NewSubscription(bufferSize),
		nextHeight:       firstHeight,
		getData:          getData,
	}
}

// Next returns the value for the next height from the subscription
func (s *HeightBasedSubscription) Next(ctx context.Context) (interface{}, error) {
	v, err := s.getData(ctx, s.nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get data for height %d: %w", s.nextHeight, err)
	}
	s.nextHeight++
	return v, nil
}
