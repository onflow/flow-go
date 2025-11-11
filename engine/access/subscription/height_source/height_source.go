package height_source

import (
	"context"

	"github.com/onflow/flow-go/engine/access/subscription"
)

type HeightSource[T any] struct {
	startHeight         uint64
	endHeight           uint64 // 0 if unbounded
	readyUpToHeightFunc func() (uint64, error)
	getItemAtHeightFunc func(ctx context.Context, height uint64) (T, error)
}

func NewHeightSource[T any](
	start uint64,
	end uint64,
	readyUpToHeightFunc func() (uint64, error),
	getItemAtHeightFunc func(ctx context.Context, height uint64) (T, error),
) *HeightSource[T] {
	return &HeightSource[T]{
		startHeight:         start,
		endHeight:           end,
		readyUpToHeightFunc: readyUpToHeightFunc,
		getItemAtHeightFunc: getItemAtHeightFunc,
	}
}

var _ subscription.HeightSource[any] = (*HeightSource[any])(nil)

func (h *HeightSource[T]) StartHeight() uint64 {
	return h.startHeight
}

func (h *HeightSource[T]) EndHeight() uint64 {
	return h.endHeight
}

func (h *HeightSource[T]) ReadyUpToHeight() (uint64, error) {
	return h.readyUpToHeightFunc()
}

func (h *HeightSource[T]) GetItemAtHeight(ctx context.Context, height uint64) (T, error) {
	var empty T

	select {
	case <-ctx.Done():
		return empty, ctx.Err()
	default:
		item, err := h.getItemAtHeightFunc(ctx, height)
		if err != nil {
			return empty, err
		}

		return item, nil
	}
}
