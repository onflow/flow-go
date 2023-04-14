package state_stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// Streamable represents a subscription that can be streamed.
type Streamable interface {
	ID() string
	Close()
	Fail(error)
	Send(context.Context, interface{}, time.Duration) error
	Next(context.Context) (interface{}, error)
}

// Streamer
type Streamer struct {
	log           zerolog.Logger
	sub           Streamable
	broadcaster   *engine.Broadcaster
	sendTimeout   time.Duration
	throttleDelay time.Duration
}

func NewStreamer(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	sendTimeout time.Duration,
	throttleDelay time.Duration,
	sub Streamable,
) *Streamer {
	return &Streamer{
		log:           log.With().Str("sub_id", sub.ID()).Logger(),
		broadcaster:   broadcaster,
		sendTimeout:   sendTimeout,
		throttleDelay: throttleDelay,
		sub:           sub,
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
		response, err := s.sub.Next(ctx)

		if err != nil {
			if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
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

		// pause before searching next response to throttle clients streaming past data.
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(s.throttleDelay):
		}
	}
}
