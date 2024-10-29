package routes

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/routes/subscription_handlers"
	"github.com/onflow/flow-go/model/flow"
)

// WSBrokerHandler handles WebSocket connections for pub/sub subscriptions.
// It upgrades incoming HTTP requests to WebSocket connections and manages the lifecycle of these connections.
// This handler uses a SubscriptionHandlerFactory to create subscription handlers that manage specific pub-sub topics.
type WSBrokerHandler struct {
	*HttpHandler

	logger            zerolog.Logger
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory
}

var _ http.Handler = (*WSBrokerHandler)(nil)

// NewWSBrokerHandler creates a new instance of WSBrokerHandler.
// It initializes the handler with the provided logger, blockchain chain, and a subHandlerFactory for subscription handlers.
//
// Parameters:
// - logger: Logger for recording internal events.
// - chain: Flow blockchain chain used for context.
// - subHandlerFactory: Factory for creating handlers that manage specific pub-sub subscriptions.
func NewWSBrokerHandler(
	logger zerolog.Logger,
	chain flow.Chain,
	subHandlerFactory *subscription_handlers.SubscriptionHandlerFactory,
) *WSBrokerHandler {
	return &WSBrokerHandler{
		logger:            logger,
		subHandlerFactory: subHandlerFactory,
		HttpHandler:       NewHttpHandler(logger, chain),
	}
}

// ServeHTTP upgrades HTTP requests to WebSocket connections and initializes pub/sub subscriptions.
// It acts as the main entry point for handling WebSocket pub/sub requests.
//
// Parameters:
// - w: The HTTP response writer.
// - r: The HTTP request being handled.
//
// Expected errors during normal operation:
// - http.StatusBadRequest: Request verification failed.
// - http.StatusInternalServerError: WebSocket upgrade error or internal issues.
func (h *WSBrokerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	logger := h.Logger.With().Str("pub_sub_subscribe_url", r.URL.String()).Logger()

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

	wsBroker := NewWebSocketBroker(
		logger,
		conn,
		//TODO: fill all limits
		LimitsConfiguration{
			activeResponsesPerSecond: atomic.NewUint64(0),
			activeSubscriptions:      atomic.NewUint64(0),
		},
		h.subHandlerFactory,
	)
	err = wsBroker.configureConnection()
	if err != nil {
		wsBroker.handleWSError(err)
		return
	}

	go wsBroker.readMessages()
	wsBroker.writeMessages()
}
