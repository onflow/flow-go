package subscription

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

// SubscriptionHandler represents common streaming data configuration for creating streaming subscription.
type SubscriptionHandler struct {
	log zerolog.Logger

	broadcaster *engine.Broadcaster

	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int
}

// NewSubscriptionHandler creates a new SubscriptionHandler instance.
//
// Parameters:
// - log: The logger to use for logging.
// - broadcaster: The engine broadcaster for publishing notifications.
// - sendTimeout: The duration after which a send operation will timeout.
// - responseLimit: The maximum allowed response time for a single stream.
// - sendBufferSize: The size of the response buffer for sending messages to the client.
//
// Returns a new SubscriptionHandler instance.
func NewSubscriptionHandler(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	sendTimeout time.Duration,
	responseLimit float64,
	sendBufferSize uint,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		log:            log,
		broadcaster:    broadcaster,
		sendTimeout:    sendTimeout,
		responseLimit:  responseLimit,
		sendBufferSize: int(sendBufferSize),
	}
}

// Subscribe creates and starts a new subscription.
//
// Parameters:
// - ctx: The context for the operation.
// - startHeight: The height to start subscription from.
// - getData: The function to retrieve data by height.
func (h *SubscriptionHandler) Subscribe(
	ctx context.Context,
	startHeight uint64,
	getData GetDataByHeightFunc,
) Subscription {
	sub := NewHeightBasedSubscription(h.sendBufferSize, startHeight, getData)
	go NewStreamer(h.log, h.broadcaster, h.sendTimeout, h.responseLimit, sub).Stream(ctx)

	return sub
}
