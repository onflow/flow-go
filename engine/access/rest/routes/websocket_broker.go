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

// Define a base struct to determine the action
type BaseMessage struct {
	Action string `json:"action"`
}

type SubscribeMessage struct {
	BaseMessage
	Topic     string                 `json:"topic"`
	Arguments map[string]interface{} `json:"arguments"`
}

type UnsubscribeMessage struct {
	BaseMessage
	Topic string `json:"topic"`
	ID    string `json:"id"`
}

type ListSubscriptionsMessage struct {
	BaseMessage
}

type LimitsConfiguration struct {
	maxSubscriptions    uint64
	activeSubscriptions *atomic.Uint64

	maxResponsesPerSecond    uint64
	activeResponsesPerSecond uint64

	sendMessageTimeout time.Duration
}

type WebSocketBroker struct {
	logger zerolog.Logger

	conn *websocket.Conn // WebSocket connection for communication with the client

	subs map[string]map[string]subscription_handlers.SubscriptionHandler // First key is the topic, second key is the subscription ID

	limitsConfiguration LimitsConfiguration // Limits on the maximum number of subscriptions per connection, responses per second, and send message timeout.

	readChannel      chan error       // Channel to read messages from the client
	broadcastChannel chan interface{} // Channel to read messages from node subscriptions
}

func NewWebSocketBroker(logger zerolog.Logger, conn *websocket.Conn, limitsConfiguration LimitsConfiguration) *WebSocketBroker {
	websocketBroker := &WebSocketBroker{
		logger:              logger,
		conn:                conn,
		limitsConfiguration: limitsConfiguration,
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
		case "subscribe":
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

			w.subscribe(subscribeMsg.Topic, subscribeMsg.Arguments)
		case "unsubscribe":
			var unsubscribeMsg UnsubscribeMessage
			if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
				err := fmt.Errorf("error parsing 'unsubscribe' message: %w", err)

				w.readChannel <- err
				continue
			}

			w.limitsConfiguration.activeSubscriptions.Add(-1)

			w.unsubscribe(unsubscribeMsg.ID)

		case "list_subscriptions":
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

// Triggered by the readMessages method when the action is subscribe. It extracts the topic from the message’s topic
// field, creates the appropriate SubscriptionHandler for the topic using the factory function CreateSubscription,
// and adds an instance of the new handler to the subs map. The client receives a notification confirming the successful subscription along with the specific ID.
func (w *WebSocketBroker) subscribe(topic string, arguments map[string]interface{}) {

}

// It is triggered by the readMessages method when the action is unsubscribe. It removes the relevant handler from
// the subs map by calling SubscriptionHandler::CloseSubscription and notifying the client of successful unsubscription.
func (w *WebSocketBroker) unsubscribe(subscriptionID string) {

}

// It is triggered by the readMessages method when the action is list_subscriptions. It gathers all active subscriptions
// for the current connection, formats the response, and sends it back to the client.
func (w *WebSocketBroker) listOfSubscriptions() {

}
