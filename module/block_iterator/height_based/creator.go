package height_based

import (
	"context"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type HeightBasedIteratorCreator struct {
	ctx     context.Context
	headers storage.Headers
}

var _ module.IteratorCreator = (*HeightBasedIteratorCreator)(nil)

func NewHeightBasedIteratorCreator(
	ctx context.Context, // for cancelling the iteration
	headers storage.Headers, // for looking up block by height
) *HeightBasedIteratorCreator {
	return &HeightBasedIteratorCreator{
		ctx:     ctx,
		headers: headers,
	}
}

func (h *HeightBasedIteratorCreator) CreateIterator(job module.IterateJob, writer module.IterateProgressWriter) (module.BlockIterator, error) {
	return NewHeightIterator(h.headers, writer, h.ctx, job)
}
