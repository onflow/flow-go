package rest

import (
	"fmt"

	"github.com/onflow/flow-go/access"
)

const blockQueryParam = "block_ids"
const eventTypeQuery = "type"

// getEvents for the provided block range or list of block IDs filtered by type.
func getEvents(r *request, backend access.API, link LinkGenerator) (interface{}, error) {
	startHeight := r.getQueryParam(startHeightQueryParam)
	endHeight := r.getQueryParam(endHeightQueryParam)
	eventType := r.getQueryParam(eventTypeQuery)
	blockIDs, err := r.getQueryParams(blockQueryParam)
	if err != nil {
		return nil, NewBadRequestError(fmt.Errorf("invalid list of block IDs: %w", err))
	}

	// if both height and one or both of start and end height are provided
	if len(blockIDs) > 0 && (startHeight != "" || endHeight != "") {
		err := fmt.Errorf("can only provide either block IDs or start and end height range")
		return nil, NewBadRequestError(err)
	}

	// if neither height nor start and end height are provided
	if len(blockIDs) == 0 && (startHeight == "" || endHeight == "") {
		err := fmt.Errorf("must provide either block IDs or start and end height range")
		return nil, NewBadRequestError(err)
	}

	if eventType == "" { // todo(sideninja) validate format with regex in the validation layer
		err := fmt.Errorf("event type must be provided")
		return nil, NewBadRequestError(err)
	}

	if len(blockIDs) > 0 {
		return getEventsForBlockIDs(blockIDs, eventType, backend, r)
	}

	return getEventsForRange(startHeight, endHeight, eventType, backend, r)
}

// getEventsForBlockIDs helper for getting events from list of block IDs.
func getEventsForBlockIDs(
	blockIDs []string,
	eventType string,
	backend access.API,
	r *request,
) (interface{}, error) {
	ids, err := toIDs(blockIDs)
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	events, err := backend.GetEventsForBlockIDs(r.Context(), eventType, ids)
	if err != nil {
		return nil, err
	}

	return blocksEventsResponse(events), nil
}

// getEventsForRange helper gets block in the block height range.
func getEventsForRange(
	startHeight string,
	endHeight string,
	eventType string,
	backend access.API,
	r *request,
) (interface{}, error) {
	// support providing end height as "sealed" or "final"
	if endHeight == finalHeightQueryParam || endHeight == sealedHeightQueryParam {
		latest, err := backend.GetLatestBlock(r.Context(), endHeight == sealedHeightQueryParam)
		if err != nil {
			return nil, err
		}

		endHeight = fmt.Sprintf("%d", latest.Header.Height)
	}

	start, err := toHeight(startHeight)
	if err != nil {
		heightError := fmt.Errorf("invalid start height: %w", err)
		return nil, NewBadRequestError(heightError)
	}
	end, err := toHeight(endHeight)
	if err != nil {
		heightError := fmt.Errorf("invalid end height: %w", err)
		return nil, NewBadRequestError(heightError)
	}

	if start > end {
		err := fmt.Errorf("start height must be less than or equal to end height")
		return nil, NewBadRequestError(err)
	}

	if end-start > MaxAllowedHeights {
		err := fmt.Errorf("height range %d exceeds maximum allowed of %d", end-start, MaxAllowedHeights)
		return nil, NewBadRequestError(err)
	}

	events, err := backend.GetEventsForHeightRange(r.Context(), eventType, start, end)
	if err != nil {
		return nil, err
	}

	return blocksEventsResponse(events), nil
}
