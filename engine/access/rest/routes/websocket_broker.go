package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/routes/subscription_handlers"
)

// Constants representing action types.
const (
	UnknownAction           = "unknown"
	SubscribeAction         = "subscribe"          // Action for subscription message
	UnsubscribeAction       = "unsubscribe"        // Action for unsubscription message
	ListSubscriptionsAction = "list_subscriptions" // Action to list active subscriptions
)

const (
	DefaultMaxSubscriptionsPerConnection = 1000             // Default maximum subscriptions per connection
	DefaultMaxResponsesPerSecond         = 100              // Default maximum responses per second
	DefaultSendMessageTimeout            = 10 * time.Second // Default timeout for sending messages
)

// BaseMessageRequest represents a base structure for incoming messages.
type BaseMessageRequest struct {
	Action string `json:"action"` // Action type of the request
}

// BaseMessageResponse represents a base structure for outgoing messages.
type BaseMessageResponse struct {
	Action       string `json:"action,omitempty"`        // Action type of the response
	Success      bool   `json:"success"`                 // Indicates success or failure
	ErrorMessage string `json:"error_message,omitempty"` // Error message, if any

}

// SubscribeMessageRequest represents a request to subscribe to a topic.
type SubscribeMessageRequest struct {
	BaseMessageRequest
	Topic     string                 `json:"topic"`     // Topic to subscribe to
	Arguments map[string]interface{} `json:"arguments"` // Additional arguments for subscription
}

// SubscribeMessageResponse represents the response to a subscription request.
type SubscribeMessageResponse struct {
	BaseMessageResponse
	Topic string `json:"topic"` // Topic of the subscription
	ID    string `json:"id"`    // Unique subscription ID
}

// UnsubscribeMessageRequest represents a request to unsubscribe from a topic.
type UnsubscribeMessageRequest struct {
	BaseMessageRequest
	Topic string `json:"topic"` // Topic to unsubscribe from
	ID    string `json:"id"`    // Unique subscription ID
}

// UnsubscribeMessageResponse represents the response to an unsubscription request.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
	Topic string `json:"topic"` // Topic of the unsubscription
	ID    string `json:"id"`    // Unique subscription ID
}

// ListSubscriptionsMessageRequest represents a request to list active subscriptions.
type ListSubscriptionsMessageRequest struct {
	BaseMessageRequest
}

// SubscriptionEntry represents an active subscription entry.
type SubscriptionEntry struct {
	Topic string `json:"topic,omitempty"` // Topic of the subscription
	ID    string `json:"id,omitempty"`    // Unique subscription ID
}

// ListSubscriptionsMessageResponse is the structure used to respond to list_subscriptions requests.
// It contains a list of active subscriptions for the current WebSocket connection.
type ListSubscriptionsMessageResponse struct {
	BaseMessageResponse
	Subscriptions []*SubscriptionEntry `json:"subscriptions,omitempty"`
}

// WebsocketConfig holds configuration for the WebSocketBroker connection.
type WebsocketConfig struct {
	MaxSubscriptionsPerConnection uint64
	MaxResponsesPerSecond         uint64

	SendMessageTimeout time.Duration
}

type WebSocketBroker struct {
	logger            zerolog.Logger
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory
	conn              *websocket.Conn // WebSocket connection for communication with the client

	subs map[string]subscription_handlers.SubscriptionHandler // First key is the subscription ID, second key is the topic

	config WebsocketConfig // Configuration for the WebSocket broker

	errChannel       chan error       // Channel for error messages
	broadcastChannel chan interface{} // Channel for broadcast messages

	activeSubscriptions      *atomic.Uint64 // Count of active subscriptions
	activeResponsesPerSecond *atomic.Uint64 // Count of responses per second
}

// NewWebSocketBroker initializes a new WebSocketBroker instance.
func NewWebSocketBroker(
	logger zerolog.Logger,
	config WebsocketConfig,
	conn *websocket.Conn,
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory,
) *WebSocketBroker {
	websocketBroker := &WebSocketBroker{
		logger:                   logger.With().Str("component", "websocket-broker").Logger(),
		conn:                     conn,
		config:                   config,
		subHandlerFactory:        subHandlerFactory,
		subs:                     make(map[string]subscription_handlers.SubscriptionHandler),
		activeResponsesPerSecond: atomic.NewUint64(0),
		activeSubscriptions:      atomic.NewUint64(0),
	}
	go websocketBroker.resetResponseLimit()

	return websocketBroker
}

// resetResponseLimit resets the response limit every second.
func (w *WebSocketBroker) resetResponseLimit() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		w.activeResponsesPerSecond.Store(0) // Reset the response count every second
	}
}

func (w *WebSocketBroker) configureConnection() error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil { // Set the initial write deadline for the first ping message
		return models.NewRestError(http.StatusInternalServerError, "Set the initial write deadline error: ", err)
	}
	if err := w.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil { // Set the initial read deadline for the first pong message
		return models.NewRestError(http.StatusInternalServerError, "Set the initial read deadline error: ", err)
	}
	// Establish a Pong handler
	w.conn.SetPongHandler(func(string) error {
		return w.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	return nil
}

// resolveWebSocketError handles WebSocket errors.
//
// If the error is an instance of models.StatusError, the function extracts the
// relevant information like status code and user message to construct the WebSocket
// close code and message. If the error is not a models.StatusError, a default
// internal server error close code and the error's message are used.
func (w *WebSocketBroker) resolveWebSocketError(err error) (int, string) {
	// rest status type error should be returned with status and user message provided
	var statusErr models.StatusError

	if errors.As(err, &statusErr) {
		wsMsg := statusErr.UserMessage()
		if statusErr.Status() == http.StatusRequestTimeout {
			return websocket.CloseGoingAway, wsMsg
		}
	}

	return websocket.CloseInternalServerErr, err.Error()
}

// handleWSError handles errors that should close the WebSocket connection gracefully.
// It retrieves the WebSocket close code and message, sends a close message to the client,
// closes read and broadcast channels, and clears all active subscriptions.
func (w *WebSocketBroker) handleWSError(err error) {
	// Get WebSocket close code and message from the error
	wsCode, wsMsg := w.resolveWebSocketError(err)

	// Send the close message to the client
	err = w.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(wsCode, wsMsg), time.Now().Add(time.Second))
	if err != nil {
		w.logger.Error().Err(err).Msgf("error sending WebSocket CloseMessage error: %v", err)
	}

	w.cleanupAfterError()
}

// readMessages runs while the connection is active. It retrieves, validates, and processes client messages.
// Actions handled include subscribe, unsubscribe, and list_subscriptions. Additional actions can be added as needed.
// It continuously reads messages from the WebSocket connection and closes
// the associated read channel when the connection is closed by client
//
// This method should be called after establishing the WebSocket connection
// to handle incoming messages asynchronously.
func (w *WebSocketBroker) readMessages() {
	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err) {
				w.logger.Info().Msg("connection closed by client")
			}
			// Send the error to the error channel for handling in writeMessages
			w.errChannel <- err
			return
		}

		// Process the incoming message
		if err := w.processMessage(message); err != nil {
			// Send structured error response on failure
			w.logger.Err(err).Msg("failed to send error message response")
		}
	}
}

// processMessage processes incoming WebSocket messages based on their action type.
func (w *WebSocketBroker) processMessage(message []byte) error {
	var baseMsg BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return w.sendErrorResponse(UnknownAction, fmt.Sprintf("invalid message structure: 'action' is required: %v", err))
	}

	var err error
	switch baseMsg.Action {
	case SubscribeAction:
		err = w.handleSubscribeRequest(message)
	case UnsubscribeAction:
		err = w.handleUnsubscribeRequest(message)
	case ListSubscriptionsAction:
		err = w.handleListSubscriptionsRequest(message)
	default:
		err = fmt.Errorf("unsupported action type: %s", baseMsg.Action)
	}
	if err != nil {
		return w.sendErrorResponse(baseMsg.Action, err.Error())
	}

	return nil
}

func (w *WebSocketBroker) handleSubscribeRequest(message []byte) error {
	var subscribeMsg SubscribeMessageRequest
	if err := json.Unmarshal(message, &subscribeMsg); err != nil {
		return fmt.Errorf("failed to parse 'subscribe' message: %w", err)
	}

	if w.activeSubscriptions.Load() >= w.config.MaxSubscriptionsPerConnection {
		return fmt.Errorf("max subscriptions reached, max subscriptions per connection count: %d", w.config.MaxSubscriptionsPerConnection)
	}
	w.activeSubscriptions.Add(1)

	return w.subscribe(&subscribeMsg)
}

func (w *WebSocketBroker) handleUnsubscribeRequest(message []byte) error {
	var unsubscribeMsg UnsubscribeMessageRequest
	if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
		return fmt.Errorf("failed to parse 'unsubscribe' message: %w", err)
	}

	w.activeSubscriptions.Sub(1)
	return w.unsubscribe(&unsubscribeMsg)
}

func (w *WebSocketBroker) handleListSubscriptionsRequest(message []byte) error {
	var listSubscriptionsMsg ListSubscriptionsMessageRequest
	if err := json.Unmarshal(message, &listSubscriptionsMsg); err != nil {
		return fmt.Errorf("failed to parse 'list_subscriptions' message: %w", err)
	}

	w.listOfSubscriptions()

	return nil
}

// writeMessages runs while the connection is active, listening on the broadcast channel.
// It retrieves responses and sends them to the client.
func (w *WebSocketBroker) writeMessages() {
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case err := <-w.errChannel:
			// we use errChannel as indicator of client's status, when errChannel closes it means that client
			// connection has been terminated, and we need to stop this goroutine to avoid memory leak.
			w.handleWSError(err)
			return
		case data, ok := <-w.broadcastChannel:
			if !ok {
				err := fmt.Errorf("broadcast channel closed, no error occurred")
				w.handleWSError(models.NewRestError(http.StatusRequestTimeout, "broadcast channel closed", err))
				return
			}

			if err := w.sendResponse(data); err != nil {
				w.handleWSError(err)
				return
			}
		case <-pingTicker.C:
			if err := w.sendPing(); err != nil {
				return
			}
		}
	}
}

// sendResponse sends a JSON message to the WebSocket client, setting the write deadline.
// Returns an error if the write fails, causing the connection to close.
func (w *WebSocketBroker) sendResponse(data interface{}) error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		w.handleWSError(models.NewRestError(http.StatusInternalServerError, "failed to set write deadline", err))
		return err
	}

	return w.conn.WriteJSON(data)
}

// Helper to send structured error responses
func (w *WebSocketBroker) sendErrorResponse(action, errMsg string) error {
	return w.sendResponse(BaseMessageResponse{
		Action:       action,
		Success:      false,
		ErrorMessage: errMsg,
	})
}

// sendPing sends a periodic ping message to the WebSocket client to keep the connection alive.
func (w *WebSocketBroker) sendPing() error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		w.handleWSError(models.NewRestError(http.StatusInternalServerError, "failed to set the initial write deadline for ping", err))
		return err
	}

	if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		w.handleWSError(err)
		return err
	}

	return nil
}

// broadcastMessage is called by each SubscriptionHandler,
// receiving formatted subscription messages and writing them to the broadcast channel.
func (w *WebSocketBroker) broadcastMessage(data interface{}) {
	if w.activeResponsesPerSecond.Load() >= w.config.MaxResponsesPerSecond {
		// TODO: recheck edge cases
		time.Sleep(w.config.SendMessageTimeout) // Adjust the sleep duration as needed
	}

	// Send the message to the broadcast channel
	w.broadcastChannel <- data
	w.activeResponsesPerSecond.Add(1)
}

// subscribe processes a request to subscribe to a specific topic. It uses the topic field in
// the message to create a SubscriptionHandler, which is then added to the `subs` map to track
// active subscriptions. A confirmation response is sent back to the client with the subscription ID.
//
// This method is triggered by the readMessages method when the action is "subscribe".
//
// Example response sent to client:
//
//	{
//	  "action": "subscribe",
//	  "topic": "example_topic",
//	  "id": "sub_id_1"
//	}
func (w *WebSocketBroker) subscribe(msg *SubscribeMessageRequest) error {
	subHandler, err := w.subHandlerFactory.CreateSubscriptionHandler(msg.Topic, msg.Arguments, w.broadcastMessage)
	if err != nil {
		w.logger.Err(err).Msg("Subscription handler creation failed")
		return fmt.Errorf("subscription handler creation failed: %w", err)
	}

	w.subs[subHandler.ID()] = subHandler

	w.broadcastMessage(SubscribeMessageResponse{
		BaseMessageResponse: BaseMessageResponse{
			Action:  SubscribeAction,
			Success: true,
		},
		Topic: subHandler.Topic(),
		ID:    subHandler.ID(),
	})

	return nil
}

// unsubscribe processes a request to cancel an active subscription, identified by its ID.
// It removes the relevant SubscriptionHandler from the `subs` map, closes the handler,
// and sends a confirmation response to the client.
//
// This method is triggered by the readMessages method when the action is "unsubscribe".
//
// Example response sent to client:
//
//	{
//	  "action": "unsubscribe",
//	  "topic": "example_topic",
//	  "id": "sub_id_1"
//	}
func (w *WebSocketBroker) unsubscribe(msg *UnsubscribeMessageRequest) error {
	sub, found := w.subs[msg.ID]
	if !found {
		errMsg := fmt.Sprintf("no subscription found for ID %s", msg.ID)
		w.logger.Info().Msg(errMsg)
		return fmt.Errorf(errMsg)
	}

	if err := sub.Close(); err != nil {
		w.logger.Err(err).Msgf("Failed to close subscription with ID %s", msg.ID)
		return fmt.Errorf("failed to close subscription with ID %s: %w", msg.ID, err)
	}

	delete(w.subs, msg.ID)
	w.broadcastMessage(UnsubscribeMessageResponse{
		BaseMessageResponse: BaseMessageResponse{
			Action:  UnsubscribeAction,
			Success: true,
		},
		Topic: sub.Topic(),
		ID:    sub.ID(),
	})

	return nil
}

// listOfSubscriptions gathers all active subscriptions for the current WebSocket connection,
// formats them into a ListSubscriptionsMessageResponse, and sends the response to the client.
//
// This method is triggered by the readMessages handler when the action "list_subscriptions" is received.
//
// Example message structure sent to the client:
//
//	{
//	  "action": "list_subscriptions",
//	  "subscriptions": [
//	    {"topic": "example_topic_1", "id": "sub_id_1"},
//	    {"topic": "example_topic_2", "id": "sub_id_2"}
//	  ]
//	}
func (w *WebSocketBroker) listOfSubscriptions() {
	response := ListSubscriptionsMessageResponse{
		BaseMessageResponse: BaseMessageResponse{
			Action:  ListSubscriptionsAction,
			Success: true,
		},
		Subscriptions: make([]*SubscriptionEntry, 0, len(w.subs)),
	}

	for id, sub := range w.subs {
		response.Subscriptions = append(response.Subscriptions, &SubscriptionEntry{
			Topic: sub.Topic(),
			ID:    id,
		})
	}

	w.broadcastMessage(response)
}

// Close channels and clear subscriptions on error
func (w *WebSocketBroker) cleanupAfterError() {
	close(w.errChannel)
	close(w.broadcastChannel)
	w.clearSubscriptions()
}

// clearSubscriptions closes each SubscriptionHandler in the subs map and
// removes all entries from the map.
func (w *WebSocketBroker) clearSubscriptions() {
	for id, sub := range w.subs {
		// Attempt to close the subscription
		if err := sub.Close(); err != nil {
			w.logger.Err(err).Msgf("Failed to close subscription with ID %s", id)
		}
		// Remove the subscription from the map
		delete(w.subs, id)
	}
}
