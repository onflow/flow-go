package retriever

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

type Local struct {
	eventsIndex *index.EventsIndex
}

var _ Retriever = (*Local)(nil)

func NewLocalEventsRetriever(index *index.EventsIndex) *Local {
	return &Local{
		eventsIndex: index,
	}
}

func (l *Local) Events(
	ctx context.Context,
	blocks []BlockMetadata,
	eventType flow.EventType,
	encoding entities.EventEncodingVersion,
) (EventsResponse, error) {
	missing := make([]BlockMetadata, 0)
	resp := make([]flow.BlockEvents, 0)

	for _, blockInfo := range blocks {
		if ctx.Err() != nil {
			return EventsResponse{}, rpc.ConvertError(ctx.Err(), "failed to get events from storage", codes.Canceled)
		}

		events, err := l.eventsIndex.ByBlockID(blockInfo.ID, blockInfo.Height)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) ||
				errors.Is(err, indexer.ErrIndexNotInitialized) {
				missing = append(missing, blockInfo)
				continue
			}
			err = fmt.Errorf("failed to get events for block %s: %w", blockInfo.ID, err)
			return EventsResponse{}, rpc.ConvertError(err, "failed to get events from storage", codes.Internal)
		}

		filteredEvents := make([]flow.Event, 0)
		for _, event := range events {
			if event.Type != eventType {
				continue
			}

			// events are encoded in CCF format in storage. convert to JSON-CDC if requested
			if encoding == entities.EventEncodingVersion_JSON_CDC_V0 {
				payload, err := convert.CcfPayloadToJsonPayload(event.Payload)
				if err != nil {
					err = fmt.Errorf("failed to convert event payload for block %s: %w", blockInfo.ID, err)
					return EventsResponse{}, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
				}
				event.Payload = payload
			}

			filteredEvents = append(filteredEvents, event)
		}

		resp = append(resp, flow.BlockEvents{
			BlockID:        blockInfo.ID,
			BlockHeight:    blockInfo.Height,
			BlockTimestamp: blockInfo.Timestamp,
			Events:         filteredEvents,
		})
	}

	return EventsResponse{
		Events:        resp,
		MissingBlocks: missing,
	}, nil
}
