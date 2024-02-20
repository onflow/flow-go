package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/utils/logging"
)

type AccountStatusesResponse struct {
	BlockID      flow.Identifier
	Events       flow.EventsList
	MessageIndex uint64
}

type AccountStatusesBackend struct {
	log            zerolog.Logger
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc
}

func (b AccountStatusesBackend) SubscribeAccountStatuses(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.StatusFilter) state_stream.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return NewFailedSubscription(err, "could not get start height")
	}

	messageIndex := counters.NewMonotonousCounter(0)

	sub := NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getAccountStatusResponseFactory(&messageIndex, filter))

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

// getAccountStatusResponseFactory returns a function function that returns the account statuses response for a given height.
func (b AccountStatusesBackend) getAccountStatusResponseFactory(messageIndex *counters.StrictMonotonousCounter, filter state_stream.StatusFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		executionData, err := b.getExecutionData(ctx, height)
		if err != nil {
			return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
		}

		events := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			events = append(events, filter.Filter(chunkExecutionData.Events)...)
		}

		b.log.Trace().
			Hex("block_id", logging.ID(executionData.BlockID)).
			Uint64("height", height).
			Msgf("sending %d events", len(events))

		response := &AccountStatusesResponse{
			BlockID:      executionData.BlockID,
			Events:       events,
			MessageIndex: messageIndex.Value(),
		}

		if ok := messageIndex.Set(messageIndex.Value() + 1); !ok {
			b.log.Debug().Msg("message index already incremented")
		}

		return response, nil
	}
}
