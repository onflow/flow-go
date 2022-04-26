package requester

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/teivah/onecontext"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type job struct {
	ctx             context.Context
	resultID        flow.Identifier
	executionDataID flow.Identifier
	blockID         flow.Identifier
	blockHeight     uint64
}

type handler struct {
	getter execution_data.ExecutionDataGetter

	jobs chan *job // TODO: unbounded

	fulfiller *fulfiller
	storage   *tracker.Storage
}

func (h *handler) submit(j *job) {

}

func (h *handler) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case j := <-h.jobs:
			h.handle(ctx, j)
		}
	}
}

func (h *handler) isRetryable(err error) bool {
	errors.As(err, &execution_data.MalformedDataError{})
	// return !(errors.Is(err, ))

	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobSizeLimitExceededError if any blob in the blob tree exceeds the maximum blob size
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blobservice
	// - MismatchedBlockIDError

	// TODO: note: should also take into account context cancelled errors

	return false
}

func (h *handler) get(ctx context.Context, j *job) (*execution_data.ExecutionData, error) {
	h.storage.ProtectTrackBlobs()
	defer h.storage.UnprotectTrackBlobs()
	return h.getter.GetExecutionData(ctx, j.executionDataID, func(cids ...cid.Cid) error {
		return h.storage.TrackBlobs(j.blockHeight, cids)
	})
}

func (h *handler) handle(ctx irrecoverable.SignalerContext, j *job) {
	// handle errors
	// TODO: how do we handle failed jobs which are actually sealed? maybe just don't?

	getCtx, cancel := onecontext.Merge(ctx, j.ctx)
	defer cancel()

tryGet:
	select {
	case <-getCtx.Done():
		// TODO: log something
		return
	default:
	}

	h.storage.ProtectTrackBlobs()
	executionData, err := h.getter.GetExecutionData(getCtx, j.executionDataID, func(cids ...cid.Cid) error {
		return h.storage.TrackBlobs(j.blockHeight, cids)
	})
	if err != nil {
		// TODO: maybe check context to see if it's a context error?
		if h.isRetryable(err) {
			// TODO: log something
			goto tryGet
		}

		// TODO: log something
		return
	}

	if executionData.BlockID != j.blockID {
		// TODO: log something
		return
	}

	h.fulfiller.submit(&completedJob{
		executionData: executionData,
		resultID:      j.resultID,
		blockHeight:   j.blockHeight,
	})
}
