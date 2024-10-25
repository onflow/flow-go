package routes

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/access/rest/routes/subscription_handlers"
)

type LimitsConfiguration struct {
	maxSubscriptions    uint64
	activeSubscriptions *atomic.Uint64

	maxResponsesPerSecond uint64
	sendMessageTimeout    time.Duration
}

type WebSocketBroker struct {
	logger zerolog.Logger

	conn *websocket.Conn // WebSocket connection for communication with the client

	subs map[string]map[string]subscription_handlers.SubscriptionHandler // First key is the topic, second key is the subscription ID

	limitsConfiguration LimitsConfiguration // Limits on the maximum number of subscriptions per connection, responses per second, and send message timeout.

	readChannel      chan interface{} // Channel to read messages from the client
	broadcastChannel chan interface{} // Channel to read messages from node subscriptions
}

func NewWebSocketBroker(logger zerolog.Logger, conn *websocket.Conn, limitsConfiguration LimitsConfiguration) *WebSocketBroker {
	return &WebSocketBroker{
		logger:              logger,
		conn:                conn,
		limitsConfiguration: limitsConfiguration,
	}
}

// TODO: I would name this SetConnectionConfig
func (w *WebSocketBroker) SetWebsocketConf() error {
	return nil
}

/*
readMessages:
This method runs while the connection is active. It retrieves, validates, and processes client messages. Actions handled include subscribe, unsubscribe, and list_subscriptions. Additional actions can be added as needed.

writeMessages:
This method runs while the connection is active, listening on the broadcast channel. It retrieves responses and sends them to the client.

broadcastMessage:
This method is called by each SubscriptionHandler, receiving formatted subscription messages and writing them to the broadcast channel.

pingPongHandler:
This method periodically checks the connection's availability using ping/pong messages and terminates the connection if the client becomes unresponsive.
*/

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
