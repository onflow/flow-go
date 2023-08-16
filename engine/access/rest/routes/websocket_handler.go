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
	request    *request.Request    // the incoming HTTP request containing the subscription details.
	respWriter http.ResponseWriter // the HTTP response writer to communicate back to the client.
	conn       *websocket.Conn     // the established WebSocket connection for communication with the client.
	api        state_stream.API    // the state_stream.API instance for managing event subscriptions.
	maxStreams int32               // the maximum number of streams allowed.
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

	decoratedRequest := request.Decorate(r, h.HttpHandler.Chain)

	subscribeHandler := SubscribeHandler{
		request:    decoratedRequest,
		respWriter: w,
		conn:       conn,
		api:        h.api,
		maxStreams: h.maxStreams,
	}

	err = subscribeHandler.SetReadWriteDeadline()
	if err != nil {
		h.errorHandler(w, err, logger)
		conn.Close()
	}

	go h.subscribeFunc(
		logger,
		subscribeHandler,
		h.eventFilterConfig,
		&h.streamCount,
		h.sendError)
}

func (h *WSHandler) sendError(
	logger zerolog.Logger,
	conn *websocket.Conn,
	err error) {
	// rest status type error should be returned with status and user message provided
	var statusErr models.StatusError
	var errMsg models.ModelError
	if errors.As(err, &statusErr) {
		errMsg = models.ModelError{
			Code:    int32(statusErr.Status()),
			Message: statusErr.UserMessage(),
		}
	} else {
		errMsg = models.ModelError{
			Code:    http.StatusInternalServerError,
			Message: "internal server error",
		}
	}

	err = conn.WriteJSON(errMsg)
	if err != nil {
		logger.Error().Err(err).Msg(fmt.Sprintf("error sending WebSocket error: %v", err))
	}
}
