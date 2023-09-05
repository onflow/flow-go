package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
)

// WebsocketController holds the necessary components and parameters for handling a WebSocket subscription.
// It manages the communication between the server and the WebSocket client for subscribing.
type WebsocketController struct {
	logger            zerolog.Logger
	conn              *websocket.Conn                // the WebSocket connection for communication with the client
	api               state_stream.API               // the state_stream.API instance for managing event subscriptions
	eventFilterConfig state_stream.EventFilterConfig // the configuration for filtering events
	maxStreams        int32                          // the maximum number of streams allowed
	streamCount       *atomic.Int32                  // the current number of active streams
	readChannel       chan interface{}               // channel which notify closing connection by the client
}

// SetWebsocketConf used to set read and write deadlines for WebSocket connections and establishes a Pong handler to
// manage incoming Pong messages. These methods allow to specify a time limit for reading from or writing to a WebSocket
// connection. If the operation (reading or writing) takes longer than the specified deadline, the connection will be closed.
func (wsController *WebsocketController) SetWebsocketConf() error {
	err := wsController.conn.SetWriteDeadline(time.Now().Add(writeWait)) // Set the initial write deadline for the first ping message
	if err != nil {
		return models.NewRestError(http.StatusInternalServerError, "Set the initial write deadline error: ", err)
	}
	err = wsController.conn.SetReadDeadline(time.Now().Add(pongWait)) // Set the initial read deadline for the first pong message
	if err != nil {
		return models.NewRestError(http.StatusInternalServerError, "Set the initial read deadline error: ", err)
	}
	// Establish a Pong handler
	wsController.conn.SetPongHandler(func(string) error {
		err := wsController.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return nil
	})
	return nil
}

// wsErrorHandler handles WebSocket errors by sending an appropriate close message
// to the client WebSocket connection.
//
// If the error is an instance of models.StatusError, the function extracts the
// relevant information like status code and user message to construct the WebSocket
// close code and message. If the error is not a models.StatusError, a default
// internal server error close code and the error's message are used.
// The connection is then closed using WriteControl to send a CloseMessage with the
// constructed close code and message. Any errors that occur during the closing
// process are logged using the provided logger.
func (wsController *WebsocketController) wsErrorHandler(err error) {
	// rest status type error should be returned with status and user message provided
	var statusErr models.StatusError
	var wsCode int
	var wsMsg string

	if errors.As(err, &statusErr) {
		if statusErr.Status() == http.StatusBadRequest {
			wsCode = websocket.CloseUnsupportedData
		}
		if statusErr.Status() == http.StatusServiceUnavailable {
			wsCode = websocket.CloseTryAgainLater
		}
		if statusErr.Status() == http.StatusRequestTimeout {
			wsCode = websocket.CloseGoingAway
		}
		wsMsg = statusErr.UserMessage()

	} else {
		wsCode = websocket.CloseInternalServerErr
		wsMsg = err.Error()
	}

	// Close the connection with the CloseError message
	err = wsController.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(wsCode, wsMsg), time.Now().Add(time.Second))
	if err != nil {
		wsController.logger.Error().Err(err).Msg(fmt.Sprintf("error sending WebSocket error: %v", err))
	}
}

// writeEvents use for writes events and pings to the WebSocket connection for a given subscription.
// It listens to the subscription's channel for events and writes them to the WebSocket connection.
// If an error occurs or the subscription channel is closed, it handles the error or termination accordingly.
// The function uses a ticker to periodically send ping messages to the client to maintain the connection.
func (wsController *WebsocketController) writeEvents(sub state_stream.Subscription) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case _, ok := <-wsController.readChannel:
			if !ok {
				return
			}
		case event, ok := <-sub.Channel():
			if !ok {
				if sub.Err() != nil {
					err := fmt.Errorf("stream encountered an error: %v", sub.Err())
					wsController.wsErrorHandler(models.NewBadRequestError(err))
					return
				}
				err := fmt.Errorf("subscription channel closed, no error occurred")
				wsController.wsErrorHandler(models.NewRestError(http.StatusRequestTimeout, "subscription channel closed", err))
				return
			}
			err := wsController.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				wsController.wsErrorHandler(models.NewRestError(http.StatusInternalServerError, "Set the initial write deadline error: ", err))
				return
			}
			// Write the response to the WebSocket connection
			err = wsController.conn.WriteJSON(event)
			if err != nil {
				wsController.wsErrorHandler(err)
				return
			}
		case <-ticker.C:
			err := wsController.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				wsController.wsErrorHandler(models.NewRestError(http.StatusInternalServerError, "Set the initial write deadline error: ", err))
				return
			}
			if err := wsController.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				wsController.wsErrorHandler(err)
				return
			}
		}
	}
}

// read function handles WebSocket messages from the client.
// It continuously reads messages from the WebSocket connection and closes
// the associated read channel when the connection is closed.
//
// This method should be called after establishing the WebSocket connection
// to handle incoming messages asynchronously.
func (wsController *WebsocketController) read() {
	// Start a goroutine to handle the WebSocket connection
	defer close(wsController.readChannel) // notify websocket about closed connection

	for {
		_, _, err := wsController.conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

// SubscribeHandlerFunc is a function that contains endpoint handling logic for subscribes, fetches necessary resources
type SubscribeHandlerFunc func(
	request *request.Request,
	ctx context.Context,
	wsController *WebsocketController,
) (state_stream.Subscription, error)

// WSHandler is websocket handler implementing custom websocket handler function and allows easier handling of errors and
// responses as it wraps functionality for handling error and responses outside of endpoint handling.
type WSHandler struct {
	*HttpHandler
	subscribeFunc SubscribeHandlerFunc

	api               state_stream.API
	eventFilterConfig state_stream.EventFilterConfig
	maxStreams        int32
	streamCount       *atomic.Int32
}

func NewWSHandler(
	logger zerolog.Logger,
	subscribeFunc SubscribeHandlerFunc,
	chain flow.Chain,
	api state_stream.API,
	eventFilterConfig state_stream.EventFilterConfig,
	maxGlobalStreams uint32,
) *WSHandler {
	handler := &WSHandler{
		subscribeFunc:     subscribeFunc,
		api:               api,
		eventFilterConfig: eventFilterConfig,
		maxStreams:        int32(maxGlobalStreams),
		streamCount:       &atomic.Int32{},
		HttpHandler:       NewHttpHandler(logger, chain),
	}

	return handler
}

// ServeHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	logger := h.Logger.With().Str("subscribe_url", r.URL.String()).Logger()

	err := h.VerifyRequest(w, r)
	if err != nil {
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.errorHandler(w, models.NewRestError(http.StatusInternalServerError, "webSocket upgrade error: ", err), logger)
		return
	}

	wsController := &WebsocketController{
		logger:            logger,
		conn:              conn,
		api:               h.api,
		eventFilterConfig: h.eventFilterConfig,
		maxStreams:        h.maxStreams,
		streamCount:       h.streamCount,
		readChannel:       make(chan interface{}),
	}

	err = wsController.SetWebsocketConf()
	if err != nil {
		wsController.wsErrorHandler(err)
		conn.Close()
	}

	if wsController.streamCount.Load() >= wsController.maxStreams {
		err := fmt.Errorf("maximum number of streams reached")
		wsController.wsErrorHandler(models.NewRestError(http.StatusServiceUnavailable, err.Error(), err))
	}
	wsController.streamCount.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		wsController.streamCount.Add(-1)
		wsController.conn.Close()
		cancel()
	}()

	sub, err := h.subscribeFunc(request.Decorate(r, h.HttpHandler.Chain), ctx, wsController)
	if err != nil {
		wsController.wsErrorHandler(err)
		return
	}

	go wsController.read()
	wsController.writeEvents(sub)
}
