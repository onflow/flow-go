package rest

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"

	"github.com/onflow/flow-go/access"
)

const blockQueryParam = "block_ids"
const eventTypeQuery = "type"

// GetEvents for the provided block range or list of block IDs filtered by type.
func GetEvents(r *request.Request, backend access.API, _ models.LinkGenerator) (interface{}, error) {
	req, err := r.GetEventsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	// if the request has block IDs provided then return events for block IDs
	var blocksEvents models.BlocksEvents
	if len(req.BlockIDs) > 0 {
		events, err := backend.GetEventsForBlockIDs(r.Context(), req.Type, req.BlockIDs)
		if err != nil {
			return nil, err
		}

		blocksEvents.Build(events)
		return blocksEvents, nil
	}

	// if end height is provided with special values then load the height
	if req.EndHeight == request.FinalHeight || req.EndHeight == request.SealedHeight {
		latest, err := backend.GetLatestBlockHeader(r.Context(), req.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		req.EndHeight = latest.Height
		// special check after we resolve special height value
		if req.StartHeight > req.EndHeight {
			return nil, NewBadRequestError(fmt.Errorf("current retrieved end height value is lower than start height"))
		}
	}

	// if request provided block height range then return events for that range
	events, err := backend.GetEventsForHeightRange(r.Context(), req.Type, req.StartHeight, req.EndHeight)
	if err != nil {
		return nil, err
	}

	blocksEvents.Build(events)
	return blocksEvents, nil
}
