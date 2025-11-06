package streamer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
)

type HeightBasedStreamer[T any] struct {
	log          zerolog.Logger
	options      StreamOptions
	limiter      *rate.Limiter
	subscription subscription.Subscription[T]
	broadcaster  *engine.Broadcaster
	heightSource subscription.HeightSource[T]
}

var _ subscription.Streamer = (*HeightBasedStreamer[any])(nil)

func NewHeightBasedStreamer[T any](
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	subscription subscription.Subscription[T],
	heightSource subscription.HeightSource[T],
	options StreamOptions,
) *HeightBasedStreamer[T] {
	var limiter *rate.Limiter
	if options.ResponseLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(options.ResponseLimit), 1)
	}

	if options.Heartbeat <= 0 {
		options.Heartbeat = 2 * time.Second
	}

	return &HeightBasedStreamer[T]{
		log:          log,
		limiter:      limiter,
		options:      options,
		broadcaster:  broadcaster,
		subscription: subscription,
		heightSource: heightSource,
	}
}

func (s *HeightBasedStreamer[T]) Stream(ctx context.Context) {
	newDataAvailableNotifier := engine.NewNotifier()
	s.broadcaster.Subscribe(newDataAvailableNotifier) //TODO: we never unsubscribe but it is expected?

	heartbeatTicker := time.NewTicker(s.options.Heartbeat) // liveness fallback
	defer heartbeatTicker.Stop()

	next := s.heightSource.StartHeight()
	end := s.heightSource.EndHeight()

	for {
		select {
		case <-ctx.Done():
			s.subscription.CloseWithError(fmt.Errorf("client disconnected: %w", ctx.Err()))
			return
		case <-newDataAvailableNotifier.Channel():
		case <-heartbeatTicker.C:
		}

		for {
			// TODO: not sure we needed bounded stream based on end height

			// EOF. all data has been sent
			if end != 0 && next > end {
				s.subscription.Close()
				return
			}

			// TODO: is it similar to sendAllAvailable ?

			readyTo, err := s.heightSource.ReadyUpToHeight()
			if err != nil {
				s.subscription.CloseWithError(err)
				return
			}
			if next > readyTo {
				break
			}

			item, err := s.heightSource.GetItemAtHeight(ctx, next)
			if err != nil {
				if errors.Is(err, subscription.ErrItemNotIngested) {
					break // not ingested yet; try again later
				}

				s.subscription.CloseWithError(err)
				return
			}

			if err := s.send(ctx, item); err != nil {
				s.subscription.CloseWithError(err)
				return
			}

			next++
		}
	}
}

func (s *HeightBasedStreamer[T]) send(ctx context.Context, value T) error {
	// if the limiter is not set, just send
	if s.limiter == nil {
		return s.subscription.Send(ctx, value, s.options.SendTimeout)
	}

	if err := s.limiter.WaitN(ctx, 1); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	return s.subscription.Send(ctx, value, s.options.SendTimeout)
}
