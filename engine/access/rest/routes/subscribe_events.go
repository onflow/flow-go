package routes

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

// SubscribeEvents create websocket connection and write to it requested events.
func SubscribeEvents(
	request *request.Request,
	ctx context.Context,
	wsController *WebsocketController) (state_stream.Subscription, error) {
	req, err := request.SubscribeEventsRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}
	// Retrieve the filter parameters from the request, if provided
	filter, err := state_stream.NewEventFilter(
		wsController.eventFilterConfig,
		request.Chain,
		req.EventTypes,
		req.Addresses,
		req.Contracts,
	)
	if err != nil {
		err := fmt.Errorf("invalid event filter")
		return nil, models.NewBadRequestError(err)
	}

	return wsController.api.SubscribeEvents(ctx, req.StartBlockID, req.StartHeight, filter), nil
}
