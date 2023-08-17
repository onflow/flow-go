package routes

import (
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
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
)

// SubscribeHandler holds the necessary components and parameters for handling a WebSocket subscription.
// It manages the communication between the server and the WebSocket client for subscribing.
type SubscribeHandler struct {
	request    *request.Request // the incoming HTTP request containing the subscription details.
	conn       *websocket.Conn  // the established WebSocket connection for communication with the client.
	api        state_stream.API // the state_stream.API instance for managing event subscriptions.
	maxStreams int32            // the maximum number of streams allowed.
}

// SetReadWriteDeadline used to set read and write deadlines for WebSocket connections. These methods allow you to
// specify a time limit for reading from or writing to a WebSocket connection. If the operation (reading or writing)
// takes longer than the specified deadline, the connection will be closed.
func (h *SubscribeHandler) SetReadWriteDeadline() error {
	err := h.conn.SetWriteDeadline(time.Now().Add(writeWait)) // Set the initial write deadline for the first ping message
	if err != nil {
		return models.NewRestError(http.StatusInternalServerError, "Set the initial write deadline error: ", err)
	}
	err = h.conn.SetReadDeadline(time.Now().Add(pongWait)) // Set the initial read deadline for the first pong message
	if err != nil {
		return models.NewRestError(http.StatusInternalServerError, "Set the initial read deadline error: ", err)
	}
	return nil
}

// SubscribeHandlerFunc is a function that contains endpoint handling logic for subscribes, fetches necessary resources
type SubscribeHandlerFunc func(
	logger zerolog.Logger,
	subscribeHandler SubscribeHandler,
	eventFilterConfig state_stream.EventFilterConfig,
	streamCount *atomic.Int32,
	errorHandler func(logger zerolog.Logger, conn *websocket.Conn, err error),
)

// WSHandler is websocket handler implementing custom websocket handler function and allows easier handling of errors and
// responses as it wraps functionality for handling error and responses outside of endpoint handling.
type WSHandler struct {
	*HttpHandler
	subscribeFunc SubscribeHandlerFunc

	api               state_stream.API
	eventFilterConfig state_stream.EventFilterConfig
	maxStreams        int32
	streamCount       atomic.Int32
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
		streamCount:       atomic.Int32{},
		HttpHandler:       NewHttpHandler(logger, chain),
	}

	return handler
}

// ServeHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	logger := h.Logger.With().Str("request_url", r.URL.String()).Logger()

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

	subscribeHandler := SubscribeHandler{
		request:    request.Decorate(r, h.HttpHandler.Chain),
		conn:       conn,
		api:        h.api,
		maxStreams: h.maxStreams,
	}

	err = subscribeHandler.SetReadWriteDeadline()
	if err != nil {
		h.wsErrorHandler(logger, conn, err)
		conn.Close()
	}

	go h.subscribeFunc(
		logger,
		subscribeHandler,
		h.eventFilterConfig,
		&h.streamCount,
		h.wsErrorHandler)
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
func (h *WSHandler) wsErrorHandler(
	logger zerolog.Logger,
	conn *websocket.Conn,
	err error) {
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
	err = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(wsCode, wsMsg), time.Now().Add(time.Second))
	if err != nil {
		logger.Error().Err(err).Msg(fmt.Sprintf("error sending WebSocket error: %v", err))
	}
}
