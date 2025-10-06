package provider

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

// LocalEventProvider retrieves events from a local execution state cache.
// It serves requests using the provided optimistic execution state, without
// contacting external execution nodes. This is useful when events are already
// available locally (e.g., after optimistic syncing).
type LocalEventProvider struct {
	execStateCache optimistic_sync.ExecutionStateCache
}

var _ EventProvider = (*LocalEventProvider)(nil)

func NewLocalEventProvider(execStateCache optimistic_sync.ExecutionStateCache) *LocalEventProvider {
	return &LocalEventProvider{
		execStateCache: execStateCache,
	}
}

func (l *LocalEventProvider) Events(
	ctx context.Context,
	blocks []BlockMetadata,
	eventType flow.EventType,
	encodingVersion entities.EventEncodingVersion,
	result *optimistic_sync.ExecutionResultInfo,
) (Response, *access.ExecutorMetadata, error) {
	if len(blocks) == 0 {
		return Response{}, nil, nil
	}

	missingBlocks := make([]BlockMetadata, 0)
	blockEvents := make([]flow.BlockEvents, 0)

	snapshot, err := l.execStateCache.Snapshot(result.ExecutionResultID)
	if err != nil {
		return Response{}, nil,
			fmt.Errorf("failed to get snapshot for execution result %s: %w", result.ExecutionResultID, err)
	}

	for _, blockInfo := range blocks {
		if ctx.Err() != nil {
			return Response{}, nil,
				rpc.ConvertError(ctx.Err(), "failed to get events from storage", codes.Canceled)
		}

		events, err := snapshot.Events().ByBlockID(blockInfo.ID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) ||
				errors.Is(err, indexer.ErrIndexNotInitialized) {
				missingBlocks = append(missingBlocks, blockInfo)
				continue
			}

			err = fmt.Errorf("failed to get events for block %s: %w", blockInfo.ID, err)
			return Response{}, nil, rpc.ConvertError(err, "failed to get events from storage", codes.Internal)
		}

		filteredEvents := make([]flow.Event, 0)
		for _, event := range events {
			if event.Type != eventType {
				continue
			}

			// events are encoded in CCF format in storage. convert to JSON-CDC if requested
			if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
				payload, err := convert.CcfPayloadToJsonPayload(event.Payload)
				if err != nil {
					err = fmt.Errorf("failed to convert event payload for block %s: %w", blockInfo.ID, err)
					return Response{}, nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
				}

				filteredEvent, err := flow.NewEvent(
					flow.UntrustedEvent{
						Type:             event.Type,
						TransactionID:    event.TransactionID,
						TransactionIndex: event.TransactionIndex,
						EventIndex:       event.EventIndex,
						Payload:          payload,
					},
				)
				if err != nil {
					return Response{}, nil, rpc.ConvertError(err, "could not construct event", codes.Internal)
				}

				event = *filteredEvent
			}

			filteredEvents = append(filteredEvents, event)
		}

		blockEvents = append(blockEvents, flow.BlockEvents{
			BlockID:        blockInfo.ID,
			BlockHeight:    blockInfo.Height,
			BlockTimestamp: blockInfo.Timestamp,
			Events:         filteredEvents,
		})
	}

	metadata := &access.ExecutorMetadata{
		ExecutionResultID: result.ExecutionResultID,
		ExecutorIDs:       result.ExecutionNodes.NodeIDs(),
	}

	response := Response{
		Events:        blockEvents,
		MissingBlocks: missingBlocks,
	}

	return response, metadata, nil
}
