package subscription

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

// SubscriptionHandler represents common streaming data configuration for access and state_stream handlers.
type SubscriptionHandler struct {
	log zerolog.Logger

	broadcaster *engine.Broadcaster

	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int
}

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

func (h *SubscriptionHandler) Subscribe(
	ctx context.Context,
	nextHeight uint64,
	getData GetDataByHeightFunc,
) Subscription {
	sub := NewHeightBasedSubscription(h.sendBufferSize, nextHeight, getData)
	go NewStreamer(h.log, h.broadcaster, h.sendTimeout, h.responseLimit, sub).Stream(ctx)

	return sub
}
