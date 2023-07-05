package state_stream

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
)

type SubscribeHandler struct {
	Api   API
	Chain flow.Chain

	EventFilterConfig EventFilterConfig

	MaxStreams  int32
	StreamCount atomic.Int32
}

func NewSubscribeHandler(api API, chain flow.Chain, conf EventFilterConfig, maxGlobalStreams uint32) *SubscribeHandler {
	h := &SubscribeHandler{
		Api:               api,
		Chain:             chain,
		EventFilterConfig: conf,
		MaxStreams:        int32(maxGlobalStreams),
		StreamCount:       atomic.Int32{},
	}
	return h
}

func (h *SubscribeHandler) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64, filter EventFilter) (Subscription, error) {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return nil, status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	return h.Api.SubscribeEvents(ctx, startBlockID, startBlockHeight, filter), nil
}
