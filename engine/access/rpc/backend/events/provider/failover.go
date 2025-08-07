package provider

import (
	"context"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

type FailoverEventProvider struct {
	log              zerolog.Logger
	localProvider    EventProvider
	execNodeProvider EventProvider
}

var _ EventProvider = (*FailoverEventProvider)(nil)

func NewFailoverEventProvider(
	log zerolog.Logger,
	localProvider EventProvider,
	execNodeProvider EventProvider,
) *FailoverEventProvider {
	return &FailoverEventProvider{
		log:              log.With().Str("event_provider", "failover").Logger(),
		localProvider:    localProvider,
		execNodeProvider: execNodeProvider,
	}
}

func (f *FailoverEventProvider) Events(
	ctx context.Context,
	blocks []BlockMetadata,
	eventType flow.EventType,
	encoding entities.EventEncodingVersion,
) (Response, error) {
	localEvents, localErr := f.localProvider.Events(ctx, blocks, eventType, encoding)
	if localErr != nil {
		f.log.Debug().Err(localErr).
			Msg("failed to get events from local storage. will try to get them from execution node")

		localEvents.MissingBlocks = blocks
	}

	if len(localEvents.MissingBlocks) == 0 {
		return localEvents, nil
	}

	f.log.Debug().
		Int("missing_blocks", len(localEvents.MissingBlocks)).
		Msg("querying execution nodes for events from missing blocks")

	execNodeEvents, execNodeErr := f.execNodeProvider.Events(ctx, localEvents.MissingBlocks, eventType, encoding)
	if execNodeErr != nil {
		return Response{}, execNodeErr
	}

	// sort ascending by block height
	// this is needed because some blocks may be retrieved from storage and others from execution nodes.
	// most likely, the earlier blocks will all be found in local storage, but that's not guaranteed,
	// especially for nodes started after a spork, or once pruning is enabled.
	// Note: this may not match the order of the original request for clients using GetEventsForBlockIDs
	// that provide out of order block IDs
	combinedEvents := append(localEvents.Events, execNodeEvents.Events...)
	sort.Slice(combinedEvents, func(i, j int) bool {
		return combinedEvents[i].BlockHeight < combinedEvents[j].BlockHeight
	})

	return Response{
		Events: combinedEvents,
	}, nil
}
