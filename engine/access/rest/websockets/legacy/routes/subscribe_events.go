package routes

import (
	"context"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/websockets/legacy"
	"github.com/onflow/flow-go/engine/access/rest/websockets/legacy/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
)

// SubscribeEvents create websocket connection and write to it requested events.
func SubscribeEvents(
	ctx context.Context,
	r *common.Request,
	wsController *legacy.WebsocketController,
) (subscription.Subscription, error) {
	req, err := request.SubscribeEventsRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}
	// Retrieve the filter parameters from the request, if provided
	filter, err := state_stream.NewEventFilter(
		wsController.EventFilterConfig,
		r.Chain,
		req.EventTypes,
		req.Addresses,
		req.Contracts,
	)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// Check if heartbeat interval was passed via request
	if req.HeartbeatInterval > 0 {
		wsController.HeartbeatInterval = req.HeartbeatInterval
	}

	return wsController.Api.SubscribeEvents(ctx, req.StartBlockID, req.StartHeight, filter), nil
}
