package routes

import (
	"fmt"

	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

const BlockQueryParam = "block_ids"
const EventTypeQuery = "type"

// GetEvents for the provided block range or list of block IDs filtered by type.
func GetEvents(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetEventsRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// if the request has block IDs provided then return events for block IDs
	var blocksEvents commonmodels.BlocksEvents
	if len(req.BlockIDs) > 0 {
		events, err := backend.GetEventsForBlockIDs(
			r.Context(),
			req.Type,
			req.BlockIDs,
			entitiesproto.EventEncodingVersion_JSON_CDC_V0,
		)
		if err != nil {
			return nil, err
		}

		blocksEvents.Build(events)
		return blocksEvents, nil
	}

	// if end height is provided with special values then load the height
	if req.EndHeight == request.FinalHeight || req.EndHeight == request.SealedHeight {
		latest, _, err := backend.GetLatestBlockHeader(r.Context(), req.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		req.EndHeight = latest.Height
		// special check after we resolve special height value
		if req.StartHeight > req.EndHeight {
			return nil, common.NewBadRequestError(fmt.Errorf("current retrieved end height value is lower than start height"))
		}
	}

	// if request provided block height range then return events for that range
	events, err := backend.GetEventsForHeightRange(
		r.Context(),
		req.Type,
		req.StartHeight,
		req.EndHeight,
		entitiesproto.EventEncodingVersion_JSON_CDC_V0,
	)
	if err != nil {
		return nil, err
	}

	blocksEvents.Build(events)
	return blocksEvents, nil
}
