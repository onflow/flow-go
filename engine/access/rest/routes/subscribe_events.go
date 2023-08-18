package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

// SubscribeEvents create websocket connection and write to it requested events.
func SubscribeEvents(
	request *request.Request,
	wsCtx *WebsocketContext) {
	req, err := request.SubscribeEventsRequest()
	if err != nil {
		wsCtx.wsErrorHandler(models.NewBadRequestError(err))
		return
	}
	// Retrieve the filter parameters from the request, if provided
	filter, err := state_stream.NewEventFilter(
		wsCtx.eventFilterConfig,
		request.Chain,
		req.EventTypes,
		req.Addresses,
		req.Contracts,
	)
	if err != nil {
		err := fmt.Errorf("event filter error")
		wsCtx.wsErrorHandler(models.NewBadRequestError(err))
		return
	}

	if wsCtx.streamCount.Load() >= wsCtx.maxStreams {
		err := fmt.Errorf("maximum number of streams reached")
		wsCtx.wsErrorHandler(models.NewRestError(http.StatusServiceUnavailable, "maximum number of streams reached", err))
		return
	}
	wsCtx.streamCount.Add(1)

	ctx := context.Background()
	sub := wsCtx.api.SubscribeEvents(ctx, req.StartBlockID, req.StartHeight, filter)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sub.Channel():
				if !ok {
					if sub.Err() != nil {
						err := fmt.Errorf("stream encountered an error: %v", sub.Err())
						wsCtx.wsErrorHandler(models.NewBadRequestError(err))
						return
					}
					err := fmt.Errorf("subscription channel closed, no error occurred")
					wsCtx.wsErrorHandler(models.NewRestError(http.StatusRequestTimeout, "subscription channel closed", err))
					return
				}
				wsCtx.send <- event
			}
		}
	}()
}
