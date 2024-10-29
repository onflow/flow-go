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
	SubscribeAction         = "subscribe"          // Action for subscription message
	UnsubscribeAction       = "unsubscribe"        // Action for unsubscription message
	ListSubscriptionsAction = "list_subscriptions" // Action to list active subscriptions
)

// Define a base struct to determine the action
type BaseMessageRequest struct {
	Action string `json:"action"`
}

type BaseMessageResponse struct {
	Action  string `json:"action"`
	Success bool   `json:"success"`
}

type SubscribeMessageRequest struct {
	BaseMessageRequest
	Topic     string                 `json:"topic"`
	Arguments map[string]interface{} `json:"arguments"`
}

// SubscribeMessageResponse represents the response to a subscription message.
// It includes the topic and a unique subscription ID.
type SubscribeMessageResponse struct {
	BaseMessageResponse
	Topic string `json:"topic"`
	ID    string `json:"id"`
}

type UnsubscribeMessageRequest struct {
	BaseMessageRequest
	Topic string `json:"topic"`
	ID    string `json:"id"`
}

// UnsubscribeMessageResponse represents the response to an unsubscription message.
// It includes the topic and subscription ID for confirmation.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
	Topic string `json:"topic"`
	ID    string `json:"id"`
}

type ListSubscriptionsMessageRequest struct {
	BaseMessageRequest
}

// SubscriptionEntry represents an active subscription entry with a specific topic and unique identifier.
type SubscriptionEntry struct {
	Topic string `json:"topic"`
	ID    string `json:"id"`
}

// ListSubscriptionsMessageResponse is the structure used to respond to list_subscriptions requests.
// It contains a list of active subscriptions for the current WebSocket connection.
type ListSubscriptionsMessageResponse struct {
	BaseMessageResponse
	Subscriptions []SubscriptionEntry `json:"subscriptions"`
}

type LimitsConfiguration struct {
	maxSubscriptions    uint64
	activeSubscriptions *atomic.Uint64

	maxResponsesPerSecond    uint64
	activeResponsesPerSecond *atomic.Uint64

	sendMessageTimeout time.Duration
}

type WebSocketBroker struct {
	logger            zerolog.Logger
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory
	conn              *websocket.Conn // WebSocket connection for communication with the client

	subs map[string]subscription_handlers.SubscriptionHandler // First key is the subscription ID, second key is the topic

	limitsConfiguration LimitsConfiguration // Limits on the maximum number of subscriptions per connection, responses per second, and send message timeout.

	errChannel       chan error       // Channel to read messages from the client
	broadcastChannel chan interface{} // Channel to read messages from node subscriptions
}

func NewWebSocketBroker(
	logger zerolog.Logger,
	conn *websocket.Conn,
	limitsConfiguration LimitsConfiguration,
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory,
) *WebSocketBroker {
	websocketBroker := &WebSocketBroker{
		logger:              logger.With().Str("component", "websocket-broker").Logger(),
		conn:                conn,
		limitsConfiguration: limitsConfiguration,
		subHandlerFactory:   subHandlerFactory,
		subs:                make(map[string]subscription_handlers.SubscriptionHandler),
	}
	go websocketBroker.resetResponseLimit()

	return websocketBroker
}

// resetResponseLimit resets the response limit every second.
func (w *WebSocketBroker) resetResponseLimit() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		w.limitsConfiguration.activeResponsesPerSecond.Store(0) // Reset the response count every second
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

		if statusErr.Status() == http.StatusServiceUnavailable {
			return websocket.CloseTryAgainLater, wsMsg
		}
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
	closeMessage := websocket.FormatCloseMessage(wsCode, wsMsg)
	err = w.conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(time.Second))
	if err != nil {
		w.logger.Error().Err(err).Msgf("error sending WebSocket CloseMessage error: %v", err)
	}

	w.cleanupAfterError()
}

func (w *WebSocketBroker) sendError(err error) {
	// Attempt to send the error message
	if err := w.conn.WriteMessage(websocket.TextMessage, []byte(err.Error())); err != nil {
		// TODO: check if we need log error of handleWSError here
		w.logger.Error().Err(err).Msgf("error sending WebSocket error: %v", err)
	}
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
		// reads messages from the WebSocket connection when
		// 1) the connection is closed by client
		// 2) unexpected message is received from the client

		_, message, err := w.conn.ReadMessage()
		if err != nil {
			//it means that client connection has been terminated, and we need to stop this goroutine
			if websocket.IsCloseError(err) {
				return
			}
			w.errChannel <- err
			continue
		}

		if err := w.processMessage(message); err != nil {
			w.errChannel <- err
		}
	}
}

// Process message based on action type
func (w *WebSocketBroker) processMessage(message []byte) error {
	var baseMsg BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return models.NewRestError(http.StatusBadRequest, "invalid message structure", err)
	}
	switch baseMsg.Action {
	case SubscribeAction:
		return w.handleSubscribeRequest(message)
	case UnsubscribeAction:
		return w.handleUnsubscribeRequest(message)
	case ListSubscriptionsAction:
		return w.handleListSubscriptionsRequest(message)
	default:
		err := fmt.Errorf("unknown action type: %s", baseMsg.Action)
		return models.NewRestError(http.StatusBadRequest, err.Error(), err)
	}
}

func (w *WebSocketBroker) handleSubscribeRequest(message []byte) error {
	var subscribeMsg SubscribeMessageRequest
	if err := json.Unmarshal(message, &subscribeMsg); err != nil {
		return models.NewRestError(http.StatusBadRequest, "failed to parse 'subscribe' message", err)
	}

	if w.limitsConfiguration.activeSubscriptions.Load() >= w.limitsConfiguration.maxSubscriptions {
		return models.NewRestError(http.StatusServiceUnavailable, "max subscriptions reached", nil)

	}
	w.limitsConfiguration.activeSubscriptions.Add(1)
	w.subscribe(&subscribeMsg)

	return nil
}

func (w *WebSocketBroker) handleUnsubscribeRequest(message []byte) error {
	var unsubscribeMsg UnsubscribeMessageRequest
	if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
		return models.NewRestError(http.StatusBadRequest, "failed to parse 'unsubscribe' message", err)
	}

	w.limitsConfiguration.activeSubscriptions.Add(-1)
	w.unsubscribe(&unsubscribeMsg)

	return nil
}

func (w *WebSocketBroker) handleListSubscriptionsRequest(message []byte) error {
	var listSubscriptionsMsg ListSubscriptionsMessageRequest
	if err := json.Unmarshal(message, &listSubscriptionsMsg); err != nil {
		return models.NewRestError(http.StatusBadRequest, "failed to parse 'list_subscriptions' message", err)
	}
	w.listOfSubscriptions()

	return nil
}

// writeMessages runs while the connection is active, listening on the broadcast channel.
// It retrieves responses and sends them to the client.
func (w *WebSocketBroker) writeMessages() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case err := <-w.errChannel:
			// we use errChannel
			// 1) as indicator of client's status, when errChannel closes it means that client
			// connection has been terminated, and we need to stop this goroutine to avoid memory leak.
			if websocket.IsCloseError(err) {
				w.handleWSError(err)
				return
			}

			// 2) as error receiver for any errors that occur during the reading process
			w.sendError(err)
		case data, ok := <-w.broadcastChannel:
			if !ok {
				err := fmt.Errorf("broadcast channel closed, no error occurred")
				w.handleWSError(models.NewRestError(http.StatusRequestTimeout, "broadcast channel closed", err))
				return
			}

			if err := w.sendData(data); err != nil {
				return
			}
		case <-ticker.C:
			if err := w.sendPing(); err != nil {
				return
			}
		}
	}
}

// sendData sends a JSON message to the WebSocket client, setting the write deadline.
// Returns an error if the write fails, causing the connection to close.
func (w *WebSocketBroker) sendData(data interface{}) error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		w.handleWSError(models.NewRestError(http.StatusInternalServerError, "failed to set write deadline", err))
		return err
	}

	if err := w.conn.WriteJSON(data); err != nil {
		w.handleWSError(err)
		return err
	}

	return nil
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
	if w.limitsConfiguration.activeResponsesPerSecond.Load() >= w.limitsConfiguration.maxResponsesPerSecond {
		// TODO: recheck edge cases
		time.Sleep(w.limitsConfiguration.sendMessageTimeout) // Adjust the sleep duration as needed
	}

	// Send the message to the broadcast channel
	w.broadcastChannel <- data
	w.limitsConfiguration.activeResponsesPerSecond.Add(1)
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
func (w *WebSocketBroker) subscribe(msg *SubscribeMessageRequest) {
	subHandler, err := w.subHandlerFactory.CreateSubscriptionHandler(msg.Topic, msg.Arguments, w.broadcastMessage)
	if err != nil {
		w.logger.Err(err).Msg("Subscription handler creation failed")
		w.sendError(err)
		return
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
func (w *WebSocketBroker) unsubscribe(msg *UnsubscribeMessageRequest) {
	sub, found := w.subs[msg.ID]
	if !found {
		w.logger.Info().Msg(fmt.Sprintf("No subscription found for ID %s", msg.ID))
		// TODO: update error
		w.sendError(fmt.Errorf("no subscription found for ID: %s", msg.ID))
		return
	}

	response := UnsubscribeMessageResponse{
		BaseMessageResponse: BaseMessageResponse{
			Action:  UnsubscribeAction,
			Success: true,
		},
		Topic: sub.Topic(),
		ID:    sub.ID(),
	}

	if err := sub.Close(); err != nil {
		response.Success = false
		w.logger.Err(err).Msgf("Failed to close subscription with ID %s", msg.ID)
		w.sendError(err)
		return
	}

	delete(w.subs, msg.ID)

	w.broadcastMessage(response)
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
		Subscriptions: make([]SubscriptionEntry, 0, len(w.subs)),
	}

	for id, sub := range w.subs {
		response.Subscriptions = append(response.Subscriptions, SubscriptionEntry{
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
