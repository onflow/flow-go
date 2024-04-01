package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
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
	activeStreamCount *atomic.Int32                  // the current number of active streams
	readChannel       chan error                     // channel which notify closing connection by the client and provide errors to the client
	heartbeatInterval uint64                         // the interval to deliver heartbeat messages to client[IN BLOCKS]
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

// writeEvents is used for writing events and pings to the WebSocket connection for a given subscription.
// It listens to the subscription's channel for events and writes them to the WebSocket connection.
// If an error occurs or the subscription channel is closed, it handles the error or termination accordingly.
// The function uses a ticker to periodically send ping messages to the client to maintain the connection.
func (wsController *WebsocketController) writeEvents(sub subscription.Subscription) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	blocksSinceLastMessage := uint64(0)
	for {
		select {
		case err := <-wsController.readChannel:
			// we use `readChannel`
			// 1) as indicator of client's status, when `readChannel` closes it means that client
			// connection has been terminated and we need to stop this goroutine to avoid memory leak.
			// 2) as error receiver for any errors that occur during the reading process
			if err != nil {
				wsController.wsErrorHandler(err)
			}
			return
		case event, ok := <-sub.Channel():
			if !ok {
				if sub.Err() != nil {
					err := fmt.Errorf("stream encountered an error: %v", sub.Err())
					wsController.wsErrorHandler(err)
					return
				}
				err := fmt.Errorf("subscription channel closed, no error occurred")
				wsController.wsErrorHandler(models.NewRestError(http.StatusRequestTimeout, "subscription channel closed", err))
				return
			}
			err := wsController.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				wsController.wsErrorHandler(models.NewRestError(http.StatusInternalServerError, "failed to set the initial write deadline: ", err))
				return
			}

			resp, ok := event.(*backend.SubscribeEventsResponse)
			if !ok {
				err = fmt.Errorf("unexpected response type: %s", event)
				wsController.wsErrorHandler(err)
				return
			}
			// responses with empty events increase heartbeat interval counter, when threshold is met a heartbeat
			// message will be emitted.
			if len(resp.Events) == 0 {
				blocksSinceLastMessage++
				if blocksSinceLastMessage < wsController.heartbeatInterval {
					continue
				}
				blocksSinceLastMessage = 0
			}

			// EventsResponse contains CCF encoded events, and this API returns JSON-CDC events.
			// convert event payload formats.
			for i, e := range resp.Events {
				payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
				if err != nil {
					err = fmt.Errorf("could not convert event payload from CCF to Json: %w", err)
					wsController.wsErrorHandler(err)
					return
				}
				resp.Events[i].Payload = payload
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
				wsController.wsErrorHandler(models.NewRestError(http.StatusInternalServerError, "failed to set the initial write deadline: ", err))
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
// the associated read channel when the connection is closed by client or when an
// any additional message is received from the client.
//
// This method should be called after establishing the WebSocket connection
// to handle incoming messages asynchronously.
func (wsController *WebsocketController) read() {
	// Start a goroutine to handle the WebSocket connection
	defer close(wsController.readChannel) // notify websocket about closed connection

	for {
		// reads messages from the WebSocket connection when
		// 1) the connection is closed by client
		// 2) a message is received from the client
		_, msg, err := wsController.conn.ReadMessage()
		if err != nil {
			if _, ok := err.(*websocket.CloseError); !ok {
				wsController.readChannel <- err
			}
			return
		}

		// Check the message from the client, if is any just close the connection
		if len(msg) > 0 {
			err := fmt.Errorf("the client sent an unexpected message, connection closed")
			wsController.logger.Debug().Msg(err.Error())
			wsController.readChannel <- err
			return
		}
	}
}

// SubscribeHandlerFunc is a function that contains endpoint handling logic for subscribes, fetches necessary resources
type SubscribeHandlerFunc func(
	ctx context.Context,
	request *request.Request,
	wsController *WebsocketController,
) (subscription.Subscription, error)

// WSHandler is websocket handler implementing custom websocket handler function and allows easier handling of errors and
// responses as it wraps functionality for handling error and responses outside of endpoint handling.
type WSHandler struct {
	*HttpHandler
	subscribeFunc SubscribeHandlerFunc

	api                      state_stream.API
	eventFilterConfig        state_stream.EventFilterConfig
	maxStreams               int32
	defaultHeartbeatInterval uint64
	activeStreamCount        *atomic.Int32
}

var _ http.Handler = (*WSHandler)(nil)

func NewWSHandler(
	logger zerolog.Logger,
	api state_stream.API,
	subscribeFunc SubscribeHandlerFunc,
	chain flow.Chain,
	stateStreamConfig backend.Config,
) *WSHandler {
	handler := &WSHandler{
		subscribeFunc:            subscribeFunc,
		api:                      api,
		eventFilterConfig:        stateStreamConfig.EventFilterConfig,
		maxStreams:               int32(stateStreamConfig.MaxGlobalStreams),
		defaultHeartbeatInterval: stateStreamConfig.HeartbeatInterval,
		activeStreamCount:        atomic.NewInt32(0),
		HttpHandler:              NewHttpHandler(logger, chain),
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
		// VerifyRequest sets the response error before returning
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	upgrader := websocket.Upgrader{
		// allow all origins by default, operators can override using a proxy
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.errorHandler(w, models.NewRestError(http.StatusInternalServerError, "webSocket upgrade error: ", err), logger)
		return
	}
	defer conn.Close()

	wsController := &WebsocketController{
		logger:            logger,
		conn:              conn,
		api:               h.api,
		eventFilterConfig: h.eventFilterConfig,
		maxStreams:        h.maxStreams,
		activeStreamCount: h.activeStreamCount,
		readChannel:       make(chan error),
		heartbeatInterval: h.defaultHeartbeatInterval, // set default heartbeat interval from state stream config
	}

	err = wsController.SetWebsocketConf()
	if err != nil {
		wsController.wsErrorHandler(err)
		return
	}

	if wsController.activeStreamCount.Load() >= wsController.maxStreams {
		err := fmt.Errorf("maximum number of streams reached")
		wsController.wsErrorHandler(models.NewRestError(http.StatusServiceUnavailable, err.Error(), err))
		return
	}
	wsController.activeStreamCount.Add(1)
	defer wsController.activeStreamCount.Add(-1)

	// cancelling the context passed into the `subscribeFunc` to ensure that when the client disconnects,
	// gorountines setup by the backend are cleaned up.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub, err := h.subscribeFunc(ctx, request.Decorate(r, h.HttpHandler.Chain), wsController)
	if err != nil {
		wsController.wsErrorHandler(err)
		return
	}

	go wsController.read()
	wsController.writeEvents(sub)
}
