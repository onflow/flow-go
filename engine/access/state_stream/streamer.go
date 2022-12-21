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

type Streamable interface {
	ID() string
	Fail(error)
	Send(context.Context, interface{}, time.Duration) error
	Next(context.Context) (interface{}, error)
}

type Streamer struct {
	log         zerolog.Logger
	broadcaster *engine.Broadcaster
	sendTimeout time.Duration
	sub         Streamable
}

func NewStreamer(
	log zerolog.Logger,
	broadcaster *engine.Broadcaster,
	sendTimeout time.Duration,
	sub Streamable,
) *Streamer {
	return &Streamer{
		log:         log.With().Str("sub_id", sub.ID()).Logger(),
		broadcaster: broadcaster,
		sendTimeout: sendTimeout,
		sub:         sub,
	}
}

func (s *Streamer) Stream(ctx context.Context) {
	s.log.Debug().Msg("starting streaming")
	defer s.log.Debug().Msg("finished streaming")

	notifier := s.broadcaster.Subscribe()
	for {
		select {
		case <-ctx.Done():
			s.sub.Fail(fmt.Errorf("client disconnected: %w", ctx.Err()))
			return
		case <-notifier.Channel():
			s.log.Trace().Msg("received broadcast notification")
		}

		err := s.sendAllAvailable(ctx)
		if err != nil {
			s.sub.Fail(err)
			return
		}
	}
}

func (s *Streamer) sendAllAvailable(ctx context.Context) error {
	for {
		response, err := s.sub.Next(ctx)

		if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
			// no more available
			return nil
		}
		if err != nil {
			return fmt.Errorf("could not get response: %w", err)
		}

		// TODO: add label that indicates the response's height/block/id
		s.log.Trace().Msg("sending response")

		err = s.sub.Send(ctx, response, s.sendTimeout)
		if err != nil {
			return err
		}
	}
}
