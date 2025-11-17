package subscription

import (
	"context"
	"fmt"
)

// GetDataByHeightFunc is a callback used by subscriptions to retrieve data for a given height.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] - if data is not found for the given height (data might not be indexed yet)
//   - [execution_data.BlobNotFoundError] - if required blobs for the execution data are missing
//     from the blob store/service (e.g., not yet available or pruned). This should be treated as
//     "not ready yet" and retried later.
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

// Next returns the value for the next height from the subscription.
//
// Expected error returns during normal operation:
//   - [subscription.ErrBlockNotReady] - the requested height is not ready yet; retry later.
//   - [storage.ErrNotFound] - if data is not found for the given height (data might not be indexed yet)
//   - [execution_data.BlobNotFoundError] - if required blobs for the execution data are missing
//     from the blob store/service (e.g., not yet available or pruned). This should be treated as
//     "not ready yet" and retried later.
//
// Note: errors are wrapped with context in this method; use errors.Is / errors.As to match
// specific errors. Context cancellations (context.Canceled/context.DeadlineExceeded) may also be
// returned as-is from the underlying call.
func (s *HeightBasedSubscription) Next(ctx context.Context) (interface{}, error) {
	v, err := s.getData(ctx, s.nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get data for height %d: %w", s.nextHeight, err)
	}
	s.nextHeight++
	return v, nil
}
