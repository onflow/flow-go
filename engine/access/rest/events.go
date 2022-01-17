package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/request"

	"github.com/onflow/flow-go/access"
)

const blockQueryParam = "block_ids"
const eventTypeQuery = "type"

// getEvents for the provided block range or list of block IDs filtered by type.
func getEvents(r *request.Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.GetEventsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	if len(req.BlockIDs) > 0 {
		events, err := backend.GetEventsForBlockIDs(r.Context(), req.Type, req.BlockIDs)
		if err != nil {
			return nil, err
		}
		// todo handle
		return
	}

	// if end height is provided with special values then load the height
	if req.EndHeight == request.FinalHeight || req.EndHeight == request.SealedHeight {
		latest, err := backend.GetLatestBlockHeader(r.Context(), req.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		req.EndHeight = latest.Height
	}

	events, err := backend.GetEventsForHeightRange(r.Context(), req.Type, req.StartHeight, req.EndHeight)
	if err != nil {
		return nil, err
	}

	// todo events response
}
