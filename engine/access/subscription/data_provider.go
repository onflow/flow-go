package subscription

import (
	"context"
	"fmt"
)

// DataProvider represents a source of sequential items to be streamed to a client.
//
// TODO: This should operate on a concrete generic type rather than `any`. It requires
// substantial refactoring and should be addressed alongside broader subscription
// package refactors in a dedicated PR.
type DataProvider interface {
	// NextData returns the next item in sequence.
	//
	// If (nil, nil) is returned, the stream progresses forward to the next item.
	//
	// Expected errors during normal operation:
	//   - [context.Canceled], [context.DeadlineExceeded]: If the context is canceled, or its deadline expires.
	//   - [ErrBlockNotReady]: If the next item is not yet available. Callers may retry later,
	//  	and the streamer will pause until new data is broadcast.
	//   - [ErrEndOfData]: If no further items are produced, the subscription is closed gracefully.
	NextData(ctx context.Context) (any, error)
}

// GetDataByHeightFunc is a callback implemented by backends and used in HeightByFuncProvider.
//
// If (nil, nil) is returned, the stream progresses forward to the next item.
//
// Expected errors during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: If the context is canceled, or its deadline expires.
//   - [ErrBlockNotReady]: If the next item is not yet available. Callers may retry later,
//     and the streamer will pause until new data is broadcast.
//   - [ErrEndOfData]: If no further items are produced, the subscription is closed gracefully.
type GetDataByHeightFunc func(ctx context.Context, height uint64) (interface{}, error)

// HeightByFuncProvider is a DataProvider that uses a GetDataByHeightFunc
// and internal height counter to sequentially fetch data by height.
//
// Deprecated: The DataProvider interface should be implemented directly by each abstraction that needs
// to provide data to a streamer/subscription.
type HeightByFuncProvider struct {
	nextHeight uint64
	getData    GetDataByHeightFunc
}

func NewHeightByFuncProvider(startHeight uint64, getData GetDataByHeightFunc) *HeightByFuncProvider {
	return &HeightByFuncProvider{
		nextHeight: startHeight,
		getData:    getData,
	}
}

var _ DataProvider = (*HeightByFuncProvider)(nil)

// NextData retrieves the next item using the configured GetDataByHeightFunc.
// It starts at the provider's current height and advances the internal height counter
// only after successful retrieval.
//
// Behavior:
//   - On success, returns the fetched value and increments the internal height by 1.
//   - On failure, returns a wrapped error and does not advance the height, so the same height
//     will be retried on the next call.
//
// If (nil, nil) is returned, the stream progresses forward to the next item.
//
// Expected errors during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: If the context is canceled, or its deadline expires.
//   - [ErrBlockNotReady]: If the next item is not yet available. Callers may retry later,
//     and the streamer will pause until new data is broadcast.
//   - [ErrEndOfData]: If no further items are produced, the subscription is closed gracefully.
func (p *HeightByFuncProvider) NextData(ctx context.Context) (any, error) {
	v, err := p.getData(ctx, p.nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get data for height %d: %w", p.nextHeight, err)
	}
	p.nextHeight++
	return v, nil
}
