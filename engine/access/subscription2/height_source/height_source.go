package height_source

import (
	"context"

	"github.com/onflow/flow-go/engine/access/subscription2"
)

type HeightSource struct {
	startHeight         uint64
	endHeight           uint64 // 0 if unbounded
	readyUpToHeightFunc func(ctx context.Context) (uint64, error)
	getItemAtHeightFunc func(ctx context.Context, height uint64) (any, error)
}

func NewHeightSource(
	start uint64,
	end uint64,
	readyUpToHeightFunc func(ctx context.Context) (uint64, error),
	getItemAtHeightFunc func(ctx context.Context, height uint64) (any, error),
) *HeightSource {
	return &HeightSource{
		startHeight:         start,
		endHeight:           end,
		readyUpToHeightFunc: readyUpToHeightFunc,
		getItemAtHeightFunc: getItemAtHeightFunc,
	}
}

var _ subscription2.HeightSource = &HeightSource{}

func (h *HeightSource) StartHeight() uint64 {
	return h.startHeight
}

func (h *HeightSource) EndHeight() uint64 {
	return h.endHeight
}

func (h *HeightSource) ReadyUpToHeight(ctx context.Context) (uint64, error) {
	//TODO: not sure this ctx and select is needed here
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		return h.readyUpToHeightFunc(ctx)
	}
}

func (h *HeightSource) GetItemAtHeight(ctx context.Context, height uint64) (any, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		return h.getItemAtHeightFunc(ctx, height)
	}
}
