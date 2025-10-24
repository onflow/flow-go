package factory

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription2"
	sub2 "github.com/onflow/flow-go/engine/access/subscription2/subscription"
)

type SubscriptionFactory struct {
	log           zerolog.Logger
	broadcaster   *engine.Broadcaster
	streamOptions subscription2.StreamOptions
}

func NewSubscriptionFactory(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	streamOptions subscription2.StreamOptions,
) *SubscriptionFactory {
	return &SubscriptionFactory{
		log:           log,
		broadcaster:   broadcaster,
		streamOptions: streamOptions,
	}
}

func (f *SubscriptionFactory) CreateHeightBasedSubscription(
	ctx context.Context,
	heightSource subscription2.HeightSource,
) subscription2.Subscription {
	sub := sub2.NewSubscription(f.streamOptions.SendBufferSize)
	streamer := subscription2.NewStreamer(f.log, f.broadcaster, sub, f.streamOptions)

	go streamer.StreamByHeight(ctx, heightSource)

	return sub
}
