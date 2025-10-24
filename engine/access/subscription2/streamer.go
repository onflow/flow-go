package subscription2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/storage"
)

type Streamer struct {
	log     zerolog.Logger
	options StreamOptions
	limiter *rate.Limiter

	subscription Subscription
	broadcaster  *engine.Broadcaster
}

type StreamOptions struct {
	SendTimeout    time.Duration
	SendBufferSize int
	RateLimitRPS   int //TODO: rename
	Heartbeat      time.Duration
}

func NewStreamer(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	subscription Subscription,
	options StreamOptions,
) *Streamer {
	var limiter *rate.Limiter
	if options.RateLimitRPS > 0 {
		limiter = rate.NewLimiter(rate.Limit(options.RateLimitRPS), 1)
	}

	if options.Heartbeat <= 0 {
		options.Heartbeat = 2 * time.Second
	}

	return &Streamer{
		log:          log,
		broadcaster:  broadcaster,
		options:      options,
		subscription: subscription,
		limiter:      limiter,
	}
}

func (s *Streamer) StreamByHeight(ctx context.Context, heightSource HeightSource) {
	notifier := engine.NewNotifier()
	s.broadcaster.Subscribe(notifier)

	ticker := time.NewTicker(s.options.Heartbeat) // liveness fallback
	defer ticker.Stop()

	next := heightSource.StartHeight()
	end := heightSource.EndHeight()

	for {
		select {
		case <-ctx.Done():
			s.subscription.CloseWithError(fmt.Errorf("client disconnected: %w", ctx.Err()))
			return
		case <-notifier.Channel():
		case <-ticker.C:
		}

		for {
			// EOF. all data has been sent
			if end != 0 && next > end {
				s.subscription.Close()
				return
			}

			readyTo, err := heightSource.ReadyUpToHeight(ctx)
			if err != nil {
				s.subscription.CloseWithError(err)
				return
			}
			if next > readyTo {
				break
			}

			item, err := heightSource.GetItemAtHeight(ctx, next)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) { //TODO: use custom sentinel
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

func (s *Streamer) send(ctx context.Context, value any) error {
	if err := s.limiter.WaitN(ctx, s.options.RateLimitRPS); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	return s.subscription.Send(ctx, value, s.options.SendTimeout)
}
