package subscription

import (
	"context"
	"fmt"
)

// DataProvider represents a source of sequential items to be streamed to a client.
//
// TODO: It can and should operate on generic type  rather than `any`. IT'd require lots of refactoring,
// so it should be done in a PR with the subscription package refactoring.
type DataProvider interface {
	// NextData retrieves the next sequential item from the data source.
	//
	// - Retrieves the next available item in sequence and returns it as the first result.
	// - If no further item is currently available yet (e.g. the next block/data is not ready), it
	//   should return a non-nil error that satisfies [ErrBlockNotReady]. The streamer interprets this
	//   as a normal "no more right now" condition and will pause until new data is broadcast.
	// - To signal a terminal end of the stream (no more data will ever be available for this
	//   subscription), return a non-nil error that satisfies [ErrEndOfData]. The streamer will close
	//   the subscription gracefully.
	// - Must observe the provided context: if the context is canceled or its deadline is exceeded,
	//   return the respective context error.
	// - Implementers should avoid returning (nil, nil). The streamer currently treats that as a
	//   no-op and immediately tries again.
	//
	// Expected errors during normal operations:
	//   - [context.Canceled], [context.DeadlineExceeded]: When the caller cancels or times out.
	//   - [ErrBlockNotReady]: When the next sequential item is not yet available.
	//   - [ErrEndOfData]: When the stream has been fully consumed and will not produce more items.
	NextData(ctx context.Context) (any, error)
}

// HeightByFuncProvider is a DataProvider that uses a GetDataByHeightFunc
// and internal height counter to sequentially fetch data by height.
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

// NextData sequentially retrieves the next item using the configured GetDataByHeightFunc.
// It starts at the provider's current height and advances the internal height counter only
// after a successful retrieval.
//
// Behavior:
//   - On success, returns the fetched value and increments the internal height by 1.
//   - On failure, returns a wrapped error and does not advance the height, so the same height
//     will be retried on the next call.
//   - Honors the provided context; context cancellation or deadline expiration should be
//     surfaced by the underlying function.
//
// Note: This implementation itself does not translate backend "not ready" conditions into
// [ErrBlockNotReady]; it simply wraps and returns them. The caller is responsible for
// interpreting such errors.
func (p *HeightByFuncProvider) NextData(ctx context.Context) (any, error) {
	v, err := p.getData(ctx, p.nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get data for height %d: %w", p.nextHeight, err)
	}
	p.nextHeight++
	return v, nil
}
