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
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

const (
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// SubscribeEvents create websocket connection and write to it requested events.
func SubscribeEvents(r *request.Request,
	w http.ResponseWriter,
	logger zerolog.Logger,
	api state_stream.API,
	eventFilterConfig state_stream.EventFilterConfig,
	maxStreams int32,
	streamCount *atomic.Int32,
	errorHandler func(w http.ResponseWriter, err error, errorLogger zerolog.Logger),
	jsonResponse func(w http.ResponseWriter, code int, response interface{}, errLogger zerolog.Logger)) {
	req, err := r.SubscribeEventsRequest()
	if err != nil {
		errorHandler(w, models.NewBadRequestError(err), logger)
		return
	}

	logger = logger.With().Str("subscribe events", r.URL.String()).Logger()
	if streamCount.Load() >= maxStreams {
		err := fmt.Errorf("maximum number of streams reached")
		errorHandler(w, models.NewRestError(http.StatusServiceUnavailable, "maximum number of streams reached", err), logger)
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r.Request, nil)
	if err != nil {
		errorHandler(w, models.NewRestError(http.StatusInternalServerError, "webSocket upgrade error: ", err), logger)
		return
	}
	// Retrieve the filter parameters from the request, if provided
	filter, err := state_stream.NewEventFilter(
		eventFilterConfig,
		r.Chain,
		req.EventTypes,
		req.Addresses,
		req.Contracts,
	)
	if err != nil {
		err := fmt.Errorf("event filter error")
		errorHandler(w, models.NewBadRequestError(err), logger)
		return
	}

	streamCount.Add(1)

	// Write messages to the WebSocket connection
	go writeEvents(logger,
		w,
		req,
		conn,
		api,
		filter,
		errorHandler,
		jsonResponse,
		streamCount)
	time.Sleep(2 * time.Second) // wait for creating child context in goroutine
}

func writeEvents(
	log zerolog.Logger,
	w http.ResponseWriter,
	req request.SubscribeEvents,
	conn *websocket.Conn,
	api state_stream.API,
	filter state_stream.EventFilter,
	errorHandler func(w http.ResponseWriter, err error, errorLogger zerolog.Logger),
	jsonResponse func(w http.ResponseWriter, code int, response interface{}, errLogger zerolog.Logger),
	streamCount *atomic.Int32,
) {
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(pingPeriod)
	sub := api.SubscribeEvents(ctx, req.StartBlockID, req.StartHeight, filter)

	defer func() {
		ticker.Stop()
		streamCount.Add(-1)
		conn.Close()
		cancel()
	}()
	err := conn.SetReadDeadline(time.Now().Add(pongWait)) // Set the initial read deadline for the first pong message
	if err != nil {
		errorHandler(w, models.NewRestError(http.StatusInternalServerError, "Set the initial read deadline error: ", err), log)
		return
	}
	conn.SetPongHandler(func(string) error {
		err = conn.SetReadDeadline(time.Now().Add(pongWait)) // Reset the read deadline upon receiving a pong message
		if err != nil {
			errorHandler(w, models.NewRestError(http.StatusInternalServerError, "Set the initial read deadline error: ", err), log)
			conn.Close()
			return err
		}
		return nil
	})

	for {
		select {
		case v, ok := <-sub.Channel():
			if !ok {
				if sub.Err() != nil {
					err := fmt.Errorf("stream encountered an error: %v", sub.Err())
					errorHandler(w, models.NewBadRequestError(err), log)
					conn.Close()
					return
				}
				err := fmt.Errorf("subscription channel closed, no error occurred")
				errorHandler(w, models.NewRestError(http.StatusRequestTimeout, "subscription channel closed", err), log)
				conn.Close()
				return
			}

			resp, ok := v.(*state_stream.EventsResponse)
			if !ok {
				err := fmt.Errorf("unexpected response type: %T", v)
				errorHandler(w, err, log)
				conn.Close()
				return
			}

			// Write the response to the WebSocket connection
			err := conn.WriteJSON(resp)
			if err != nil {
				errorHandler(w, err, log)
				conn.Close()
				return
			}
			jsonResponse(w, http.StatusOK, "", log)
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				conn.Close()
				return
			}
		}
	}
}
