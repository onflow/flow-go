package provider

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

type LocalEventProvider struct {
	//TODO: should indexer be removed completely?
	// or does it make sense to write a new indexer with exec result cache like a thin wrapper around it?
	index                   *index.EventsIndex
	execResultQueryProvider optimistic_sync.ExecutionResultQueryProvider
	execStateCache          optimistic_sync.ExecutionStateCache
}

var _ EventProvider = (*LocalEventProvider)(nil)

func NewLocalEventProvider(
	index *index.EventsIndex,
	execResultQueryProvider optimistic_sync.ExecutionResultQueryProvider,
	execStateCache optimistic_sync.ExecutionStateCache,
) *LocalEventProvider {
	return &LocalEventProvider{
		index:                   index,
		execResultQueryProvider: execResultQueryProvider,
		execStateCache:          execStateCache,
	}
}

func (l *LocalEventProvider) Events(
	ctx context.Context,
	blocks []BlockMetadata,
	eventType flow.EventType,
	encoding entities.EventEncodingVersion,
) (Response, error) {
	missing := make([]BlockMetadata, 0)
	resp := make([]flow.BlockEvents, 0)

	for _, blockInfo := range blocks {
		if ctx.Err() != nil {
			return Response{}, rpc.ConvertError(ctx.Err(), "failed to get events from storage", codes.Canceled)
		}

		query, err := l.execResultQueryProvider.ExecutionResultQuery(blockInfo.ID, optimistic_sync.Criteria{
			AgreeingExecutorsCount: 2,
			RequiredExecutors:      []flow.Identifier{},
		})
		if err != nil {
			return Response{}, err
		}

		snapshot, err := l.execStateCache.Snapshot(query.ExecutionResult.ID())
		if err != nil {
			return Response{}, fmt.Errorf("failed to get snapshot for execution result %s: %w", query.ExecutionResult.ID(), err)
		}

		events, err := snapshot.Events().ByBlockID(blockInfo.ID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) ||
				errors.Is(err, indexer.ErrIndexNotInitialized) {
				missing = append(missing, blockInfo)
				continue
			}
			err = fmt.Errorf("failed to get events for block %s: %w", blockInfo.ID, err)
			return Response{}, rpc.ConvertError(err, "failed to get events from storage", codes.Internal)
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
					return Response{}, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
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

	return Response{
		Events:        resp,
		MissingBlocks: missing,
	}, nil
}
