package subscription

import (
	"context"
	"fmt"
)

// DataProvider is a port that abstracts the retrieval of sequential data items for streaming subscriptions.
//
// In hexagonal architecture terms, this interface acts as a "driven port" (or secondary port) that
// defines how the subscription/streaming core (the hexagon) retrieves data from external sources.
// Concrete implementations serve as "adapters" that connect specific data sources (e.g., execution data,
// block headers, account statuses) to the streaming infrastructure.
//
// The Streamer component depends on this port to fetch data items sequentially, decoupling the
// streaming logic from the specifics of how and where data is retrieved. This allows:
//   - The streaming core to remain agnostic of data source implementations
//   - Easy testing via mock adapters
//   - Flexible addition of new data sources without modifying the streaming infrastructure
//
// Adapter implementations should:
//   - Maintain internal state to track the current position in the data sequence (e.g., block height)
//   - Advance to the next item only after successful retrieval
//   - Return appropriate sentinel errors to signal stream state (ErrBlockNotReady, ErrEndOfData)
//
// TODO: This should operate on a concrete generic type rather than `any` type. (issue #8093)
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
