package routes

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

// SubscribeEvents create websocket connection and write to it requested events.
func SubscribeEvents(
	logger zerolog.Logger,
	h SubscribeHandler,
	eventFilterConfig state_stream.EventFilterConfig,
	streamCount *atomic.Int32,
	errorHandler func(logger zerolog.Logger, conn *websocket.Conn, err error)) {
	logger = logger.With().Str("subscribe events", h.request.URL.String()).Logger()
	defer func() {
		h.conn.Close()
	}()

	req, err := h.request.SubscribeEventsRequest()
	if err != nil {
		errorHandler(logger, h.conn, models.NewBadRequestError(err))
		return
	}
	// Retrieve the filter parameters from the request, if provided
	filter, err := state_stream.NewEventFilter(
		eventFilterConfig,
		h.request.Chain,
		req.EventTypes,
		req.Addresses,
		req.Contracts,
	)
	if err != nil {
		err := fmt.Errorf("event filter error")
		errorHandler(logger, h.conn, models.NewBadRequestError(err))
		return
	}

	if streamCount.Load() >= h.maxStreams {
		err := fmt.Errorf("maximum number of streams reached")
		errorHandler(logger, h.conn, models.NewRestError(http.StatusServiceUnavailable, "maximum number of streams reached", err))
		return
	}
	streamCount.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()
	sub := h.api.SubscribeEvents(ctx, req.StartBlockID, req.StartHeight, filter)

	// Write messages to the WebSocket connection
	err = writeEvents(
		sub,
		h,
		streamCount)
	if err != nil {
		errorHandler(logger, h.conn, err)
		return
	}
}

// writeEvents use for writes events and pings to the WebSocket connection. It listens to a subscription's channel for
// events and writes them to the connection. If an error occurs or the subscription channel is closed, it handles the
// error or termination accordingly.
// The function uses a ticker to periodically send ping messages to the client to maintain the connection.
func writeEvents(
	sub state_stream.Subscription,
	h SubscribeHandler,
	streamCount *atomic.Int32,
) error {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		streamCount.Add(-1)
	}()

	for {
		select {
		case v, ok := <-sub.Channel():
			if !ok {
				if sub.Err() != nil {
					err := fmt.Errorf("stream encountered an error: %v", sub.Err())
					return models.NewBadRequestError(err)
				}
				err := fmt.Errorf("subscription channel closed, no error occurred")
				return models.NewRestError(http.StatusRequestTimeout, "subscription channel closed", err)
			}

			resp, ok := v.(*state_stream.EventsResponse)
			if !ok {
				return fmt.Errorf("unexpected response type: %T", v)
			}

			// Write the response to the WebSocket connection
			err := h.conn.WriteJSON(resp)
			if err != nil {
				return err
			}
		case <-ticker.C:
			if err := h.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return err
			}
		}
	}
}
