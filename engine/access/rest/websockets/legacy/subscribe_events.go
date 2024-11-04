package legacy

import (
	"context"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
)

// SubscribeEvents create websocket connection and write to it requested events.
func SubscribeEvents(
	ctx context.Context,
	request *common.Request,
	wsController *WebsocketController,
) (subscription.Subscription, error) {
	req, err := request.SubscribeEventsRequest()
	if err != nil {
		return nil, common.NewBadRequestError(err)
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
		return nil, common.NewBadRequestError(err)
	}

	// Check if heartbeat interval was passed via request
	if req.HeartbeatInterval > 0 {
		wsController.heartbeatInterval = req.HeartbeatInterval
	}

	return wsController.api.SubscribeEvents(ctx, req.StartBlockID, req.StartHeight, filter), nil
}
