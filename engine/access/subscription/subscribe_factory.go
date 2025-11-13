package subscription

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

// Factory represents common streaming data configuration for creating streaming subscription.
type Factory struct {
	log zerolog.Logger

	broadcaster *engine.Broadcaster

	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int
}

// NewSubscriptionFactory creates a new Factory instance.
//
// Parameters:
// - log: The logger to use for logging.
// - broadcaster: The engine broadcaster for publishing notifications.
// - sendTimeout: The duration after which a send operation will time out.
// - responseLimit: The maximum allowed response time for a single stream.
// - sendBufferSize: The size of the response buffer for sending messages to the client.
//
// Returns a new Factory instance.
func NewSubscriptionFactory(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	sendTimeout time.Duration,
	responseLimit float64,
	sendBufferSize uint,
) *Factory {
	return &Factory{
		log:            log,
		broadcaster:    broadcaster,
		sendTimeout:    sendTimeout,
		responseLimit:  responseLimit,
		sendBufferSize: int(sendBufferSize),
	}
}

// CreateHeightBasedSubscription creates and starts a new subscription.
//
// Parameters:
// - ctx: The context for the operation.
// - startHeight: The height to start a subscription from.
// - getData: The function to retrieve data by height.
func (h *Factory) CreateHeightBasedSubscription(
	ctx context.Context,
	startHeight uint64,
	getData GetDataByHeightFunc,
) Subscription {
	sub := NewHeightBasedSubscription(h.sendBufferSize, startHeight, getData)
	streamer := NewStreamer(h.log, h.broadcaster, h.sendTimeout, h.responseLimit, sub)

	go streamer.Stream(ctx)

	return sub
}
