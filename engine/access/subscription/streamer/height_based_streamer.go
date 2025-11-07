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
	options      *StreamOptions
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
	options *StreamOptions,
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

	// helper: send all currently available items before blocking
	sendReady := func() (done bool) {
		for {
			// EOF. all data has been sent
			if end != 0 && next > end {
				s.subscription.Close()
				return true
			}

			readyTo, err := s.heightSource.ReadyUpToHeight()
			if err != nil {
				s.subscription.CloseWithError(err)
				return true
			}
			if next > readyTo {
				return false
			}

			item, err := s.heightSource.GetItemAtHeight(ctx, next)
			if err != nil {
				if errors.Is(err, subscription.ErrBlockNotReady) {
					return false // not ingested yet; try again later
				}
				// Treat end-of-data as a graceful completion
				if errors.Is(err, subscription.ErrEndOfData) {
					s.subscription.Close()
					return true
				}
				s.subscription.CloseWithError(err)
				return true
			}

			if err := s.send(ctx, item); err != nil {
				s.subscription.CloseWithError(err)
				return true
			}

			next++
		}
	}

	// Drain available items immediately on start so consumers don't wait for a notifier/heartbeat
	if done := sendReady(); done {
		return
	}

	for {
		select {
		case <-ctx.Done():
			s.subscription.CloseWithError(fmt.Errorf("client disconnected: %w", ctx.Err()))
			return
		case <-newDataAvailableNotifier.Channel():
		case <-heartbeatTicker.C:
		}

		if done := sendReady(); done {
			return
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
