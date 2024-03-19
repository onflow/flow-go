package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// ErrBlockNotReady represents an error indicating that a block is not yet available or ready.
var ErrBlockNotReady = errors.New("block not ready")

// ErrEndOfData represents an error indicating that no more data available for streaming.
var ErrEndOfData = errors.New("end of data")

// ErrResponseNotAvailableForBlock represents an error indicating that the response is not available for the given block.
var ErrResponseNotAvailableForBlock = errors.New("response not available for block")

// Streamer represents a streaming subscription that delivers data to clients.
type Streamer struct {
	log         zerolog.Logger
	sub         Streamable
	broadcaster *engine.Broadcaster
	sendTimeout time.Duration
	limiter     *rate.Limiter
}

// NewStreamer creates a new Streamer instance.
func NewStreamer(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	sendTimeout time.Duration,
	limit float64,
	sub Streamable,
) *Streamer {
	var limiter *rate.Limiter
	if limit > 0 {
		// allows for 1 response per call, averaging `limit` responses per second over longer time frames
		limiter = rate.NewLimiter(rate.Limit(limit), 1)
	}

	return &Streamer{
		log:         log.With().Str("sub_id", sub.ID()).Logger(),
		broadcaster: broadcaster,
		sendTimeout: sendTimeout,
		limiter:     limiter,
		sub:         sub,
	}
}

// Stream is a blocking method that streams data to the subscription until either the context is
// cancelled or it encounters an error.
func (s *Streamer) Stream(ctx context.Context) {
	s.log.Debug().Msg("starting streaming")
	defer s.log.Debug().Msg("finished streaming")

	notifier := engine.NewNotifier()
	s.broadcaster.Subscribe(notifier)

	// always check the first time. This ensures that streaming continues to work even if the
	// execution sync is not functioning (e.g. on a past spork network, or during an temporary outage)
	notifier.Notify()

	for {
		select {
		case <-ctx.Done():
			s.sub.Fail(fmt.Errorf("client disconnected: %w", ctx.Err()))
			return
		case <-notifier.Channel():
			s.log.Debug().Msg("received broadcast notification")
		}

		err := s.sendAllAvailable(ctx)

		if err != nil {
			s.log.Err(err).Msg("error sending response")
			s.sub.Fail(err)
			return
		}
	}
}

// sendAllAvailable reads data from the streamable and sends it to the client until no more data is available.
func (s *Streamer) sendAllAvailable(ctx context.Context) error {
	for {
		// blocking wait for the streamer's rate limit to have available capacity
		if err := s.checkRateLimit(ctx); err != nil {
			return fmt.Errorf("error waiting for response capacity: %w", err)
		}

		response, err := s.sub.Next(ctx)

		if err != nil {
			if errors.Is(err, ErrResponseNotAvailableForBlock) {
				continue
			}

			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) ||
				execution_data.IsBlobNotFoundError(err) ||
				errors.Is(err, ErrBlockNotReady) {
				// no more available
				return nil
			}

			return fmt.Errorf("could not get response: %w", err)
		}

		if ssub, ok := s.sub.(*HeightBasedSubscription); ok {
			s.log.Trace().
				Uint64("next_height", ssub.nextHeight).
				Msg("sending response")
		}

		err = s.sub.Send(ctx, response, s.sendTimeout)
		if err != nil {
			return err
		}
	}
}

// checkRateLimit checks the stream's rate limit and blocks until there is room to send a response.
// An error is returned if the context is canceled or the expected wait time exceeds the context's
// deadline.
func (s *Streamer) checkRateLimit(ctx context.Context) error {
	if s.limiter == nil {
		return nil
	}

	return s.limiter.WaitN(ctx, 1)
}
