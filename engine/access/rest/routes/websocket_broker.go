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
type BaseMessage struct {
	Action string `json:"action"`
}

type BaseMessageResponse struct {
	Action  string `json:"action"`
	Success bool   `json:"success"`
}

type SubscribeMessage struct {
	BaseMessage
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

type UnsubscribeMessage struct {
	BaseMessage
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

type ListSubscriptionsMessage struct {
	BaseMessage
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
	activeResponsesPerSecond uint64

	sendMessageTimeout time.Duration
}

type WebSocketBroker struct {
	logger            zerolog.Logger
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory
	conn              *websocket.Conn // WebSocket connection for communication with the client

	subs map[string]subscription_handlers.SubscriptionHandler // First key is the subscription ID, second key is the topic

	limitsConfiguration LimitsConfiguration // Limits on the maximum number of subscriptions per connection, responses per second, and send message timeout.

	readChannel      chan error       // Channel to read messages from the client
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
	websocketBroker.startResponseLimiter()

	return websocketBroker
}

func (w *WebSocketBroker) startResponseLimiter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		w.limitsConfiguration.activeResponsesPerSecond = 0 // Reset the response count every second
	}
}

func (w *WebSocketBroker) SetConnectionConfig() error {
	err := w.conn.SetWriteDeadline(time.Now().Add(writeWait)) // Set the initial write deadline for the first ping message
	if err != nil {
		return models.NewRestError(http.StatusInternalServerError, "Set the initial write deadline error: ", err)
	}
	err = w.conn.SetReadDeadline(time.Now().Add(pongWait)) // Set the initial read deadline for the first pong message
	if err != nil {
		return models.NewRestError(http.StatusInternalServerError, "Set the initial read deadline error: ", err)
	}
	// Establish a Pong handler
	w.conn.SetPongHandler(func(string) error {
		err := w.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return nil
	})

	return nil
}

// readMessages runs while the connection is active. It retrieves, validates, and processes client messages.
// Actions handled include subscribe, unsubscribe, and list_subscriptions. Additional actions can be added as needed.
// It continuously reads messages from the WebSocket connection and closes
// the associated read channel when the connection is closed by client
//
// This method should be called after establishing the WebSocket connection
// to handle incoming messages asynchronously.
func (w *WebSocketBroker) readMessages() {
	// Start a goroutine to handle the WebSocket connection
	defer close(w.readChannel) // notify websocket about closed connection

	for {
		// reads messages from the WebSocket connection when
		// 1) the connection is closed by client
		// 2) unexpected message is received from the client

		// Step 1: Read JSON message into a byte slice
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			var closeError *websocket.CloseError
			w.readChannel <- err

			//it means that client connection has been terminated, and we need to stop this goroutine
			if errors.As(err, &closeError) {
				return
			}
		}

		var baseMsg BaseMessage
		if err := json.Unmarshal(message, &baseMsg); err != nil {
			//process the case when message does not have "action"
			err := fmt.Errorf("invalid message: message does not have 'action' field : %w", err)

			w.readChannel <- err
			continue
		}

		// Process based on the action type
		switch baseMsg.Action {
		case SubscribeAction:
			var subscribeMsg SubscribeMessage
			if err := json.Unmarshal(message, &subscribeMsg); err != nil {
				err := fmt.Errorf("error parsing 'subscribe' message: %w", err)

				w.readChannel <- err
				continue
			}

			if w.limitsConfiguration.activeSubscriptions.Load() >= w.limitsConfiguration.maxSubscriptions {
				err := fmt.Errorf("maximum number of streams reached")
				err = models.NewRestError(http.StatusServiceUnavailable, err.Error(), err)
				w.readChannel <- err

				continue
			}
			w.limitsConfiguration.activeSubscriptions.Add(1)

			w.subscribe(&subscribeMsg)
		case UnsubscribeAction:
			var unsubscribeMsg UnsubscribeMessage
			if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
				err := fmt.Errorf("error parsing 'unsubscribe' message: %w", err)

				w.readChannel <- err
				continue
			}

			w.limitsConfiguration.activeSubscriptions.Add(-1)

			w.unsubscribe(&unsubscribeMsg)

		case ListSubscriptionsAction:
			var listSubscriptionsMsg ListSubscriptionsMessage
			if err := json.Unmarshal(message, &listSubscriptionsMsg); err != nil {
				err := fmt.Errorf("error parsing 'unsubsclist_subscriptionsribe' message: %w", err)

				w.readChannel <- err
				continue
			}
			w.listOfSubscriptions()

		default:
			err := fmt.Errorf("unknown action type: %s", baseMsg.Action)

			w.readChannel <- err
			continue
		}
	}
}

// writeMessages runs while the connection is active, listening on the broadcast channel.
// It retrieves responses and sends them to the client.
func (w *WebSocketBroker) writeMessages() {
	for {
		select {
		case err := <-w.readChannel:
			// we use `readChannel`
			// 1) as indicator of client's status, when `readChannel` closes it means that client
			// connection has been terminated, and we need to stop this goroutine to avoid memory leak.

			var closeError *websocket.CloseError
			if errors.As(err, &closeError) {
				// TODO: write with "close"

				err = w.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeError.Code, closeError.Error()), time.Now().Add(time.Second))
				if err != nil {
					w.logger.Error().Err(err).Msg(fmt.Sprintf("error sending WebSocket error: %v", err))
				}

				return
			}

			// 2) as error receiver for any errors that occur during the reading process
			err = w.conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				//w.wsErrorHandler(err)
				return
			}
		case data, ok := <-w.broadcastChannel:
			if !ok {
				_ = fmt.Errorf("broadcast channel closed, no error occurred")
				//w.wsErrorHandler(models.NewRestError(http.StatusRequestTimeout, "broadcast channel closed", err))
				return
			}
			err := w.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				//w.wsErrorHandler(models.NewRestError(http.StatusInternalServerError, "failed to set the initial write deadline: ", err))
				return
			}

			// Write the response to the WebSocket connection
			err = w.conn.WriteJSON(data)
			if err != nil {
				//w.wsErrorHandler(err)
				return
			}
		}
	}
}

// broadcastMessage is called by each SubscriptionHandler,
// receiving formatted subscription messages and writing them to the broadcast channel.
func (w *WebSocketBroker) broadcastMessage(data interface{}) {
	if w.limitsConfiguration.activeResponsesPerSecond >= w.limitsConfiguration.maxResponsesPerSecond {
		time.Sleep(w.limitsConfiguration.sendMessageTimeout) // Adjust the sleep duration as needed
	}

	// Send the message to the broadcast channel
	w.broadcastChannel <- data
	w.limitsConfiguration.activeResponsesPerSecond++
}

// pingPongHandler periodically checks the connection's availability using ping/pong messages and
// terminates the connection if the client becomes unresponsive.
func (w *WebSocketBroker) pingPongHandler() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := w.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				//w.wsErrorHandler(models.NewRestError(http.StatusInternalServerError, "failed to set the initial write deadline: ", err))
				return
			}
			if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				//w.wsErrorHandler(err)
				return
			}
		case err := <-w.readChannel:
			//it means that client connection has been terminated, and we need to stop this goroutine
			var closeError *websocket.CloseError
			if errors.As(err, &closeError) {
				return
			}
		}
	}
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
func (w *WebSocketBroker) subscribe(msg *SubscribeMessage) {
	subHandler, err := w.subHandlerFactory.CreateSubscriptionHandler(msg.Topic, msg.Arguments, w.broadcastMessage)
	if err != nil {
		// TODO: Handle as client error response
		w.logger.Err(err).Msg("Subscription handler creation failed")
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
func (w *WebSocketBroker) unsubscribe(msg *UnsubscribeMessage) {
	sub, found := w.subs[msg.ID]
	if !found {
		// TODO: Handle as client error response
		w.logger.Info().Msg(fmt.Sprintf("No subscription found for ID %s", msg.ID))
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
		// TODO: Handle as client error response
		response.Success = false
		w.logger.Err(err).Msgf("Failed to close subscription with ID %s", msg.ID)
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
