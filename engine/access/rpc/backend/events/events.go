package events

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/events/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DefaultMaxHeightRange is the default maximum size of range requests.
const DefaultMaxHeightRange = 250

type Events struct {
	headers            storage.Headers
	state              protocol.State
	chain              flow.Chain
	maxHeightRange     uint
	provider           provider.EventProvider
	queryMode          query_mode.IndexQueryMode
	execResultProvider optimistic_sync.ExecutionResultInfoProvider
}

var _ access.EventsAPI = (*Events)(nil)

func NewEventsBackend(
	log zerolog.Logger,
	state protocol.State,
	chain flow.Chain,
	maxHeightRange uint,
	headers storage.Headers,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	queryMode query_mode.IndexQueryMode,
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) (*Events, error) {
	var eventProvider provider.EventProvider

	switch queryMode {
	case query_mode.IndexQueryModeLocalOnly:
		eventProvider = provider.NewLocalEventProvider(executionStateCache)

	case query_mode.IndexQueryModeExecutionNodesOnly:
		eventProvider = provider.NewENEventProvider(
			log,
			execNodeIdentitiesProvider,
			connFactory,
			nodeCommunicator,
		)

	case query_mode.IndexQueryModeFailover:
		local := provider.NewLocalEventProvider(executionStateCache)
		execNode := provider.NewENEventProvider(
			log,
			execNodeIdentitiesProvider,
			connFactory,
			nodeCommunicator,
		)
		eventProvider = provider.NewFailoverEventProvider(log, local, execNode)

	default:
		return nil, fmt.Errorf("unknown execution mode: %v", queryMode)
	}

	return &Events{
		state:              state,
		chain:              chain,
		maxHeightRange:     maxHeightRange,
		headers:            headers,
		provider:           eventProvider,
		queryMode:          queryMode,
		execResultProvider: executionResultProvider,
	}, nil
}

// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and
// the end block height (inclusive) that have the given type.
func (e *Events) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error) {
	if _, err := events.ValidateEvent(flow.EventType(eventType), e.chain); err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid event type: %v", err)
	}

	if endHeight < startHeight {
		return nil, nil, status.Error(codes.InvalidArgument, "start height must not be larger than end height")
	}

	rangeSize := endHeight - startHeight + 1 // range is inclusive on both ends
	if rangeSize > uint64(e.maxHeightRange) {
		return nil, nil, status.Errorf(codes.InvalidArgument, "requested block range (%d) exceeded maximum (%d)",
			rangeSize, e.maxHeightRange)
	}

	// get the latest sealed block header
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		// sealed block must be in the store, so throw an exception for any error
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}

	// a start height should not be beyond the last sealed height
	if startHeight > sealed.Height {
		return nil, nil,
			status.Errorf(codes.OutOfRange, "start height %d is greater than the last sealed block height %d", startHeight, sealed.Height)
	}

	// limit max height to the last sealed block in the chain
	//
	// Note: this causes unintuitive behavior for clients making requests through a proxy that
	// fronts multiple nodes. With that setup, clients may receive responses for a smaller range
	// than requested because the node serving the request has a slightly delayed view of the chain.
	//
	// An alternative option is to return an error here, but that's likely to cause more pain for
	// these clients since the requests would intermittently fail. it's recommended instead to
	// check the block height of the last message in the response. this will be the last block
	// height searched, and can be used to determine the start height for the next range.
	if endHeight > sealed.Height {
		endHeight = sealed.Height
	}

	// find the block headers for all the blocks between min and max height (inclusive)
	blockHeaders := make([]provider.BlockMetadata, 0, endHeight-startHeight+1)

	for i := startHeight; i <= endHeight; i++ {
		// this looks inefficient, but it's actually what's done under the covers by `headers.ByHeight`
		// and avoids calculating header.ID() for each block.
		blockID, err := e.headers.BlockIDByHeight(i)
		if err != nil {
			return nil, nil, rpc.ConvertStorageError(common.ResolveHeightError(e.state.Params(), i, err))
		}
		header, err := e.headers.ByBlockID(blockID)
		if err != nil {
			return nil, nil, rpc.ConvertStorageError(fmt.Errorf("failed to get block header for %d: %w", i, err))
		}

		blockHeaders = append(blockHeaders, provider.BlockMetadata{
			ID:        blockID,
			Height:    header.Height,
			Timestamp: time.UnixMilli(int64(header.Timestamp)).UTC(),
		})
	}

	// get the result for the block with the highest height. all data queried for this set of blocks
	// must be from the execution fork terminating at this result. this guarantees the response
	// contains a consistent view of the state.
	lastBlockID := blockHeaders[len(blockHeaders)-1].ID
	execResultInfo, err := e.execResultProvider.ExecutionResultInfo(lastBlockID, criteria)
	if err != nil {
		return nil, nil,
			fmt.Errorf("failed to get execution result for last block: %w", err)
	}

	resp, metadata, err := e.provider.Events(
		ctx,
		blockHeaders,
		flow.EventType(eventType),
		requiredEventEncodingVersion,
		execResultInfo,
	)
	if err != nil {
		return nil, metadata, err
	}

	return resp.Events, metadata, nil
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (e *Events) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error) {
	if len(blockIDs) == 0 {
		return nil, nil, status.Error(codes.InvalidArgument, "block IDs must not be empty")
	}

	if uint(len(blockIDs)) > e.maxHeightRange {
		return nil, nil,
			status.Errorf(codes.InvalidArgument, "requested block range (%d) exceeded maximum (%d)",
				len(blockIDs), e.maxHeightRange)
	}

	if _, err := events.ValidateEvent(flow.EventType(eventType), e.chain); err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid event type: %v", err)
	}

	var newestView uint64
	var newestBlockID flow.Identifier

	// deduplicate block IDs while preserving order
	seen := make(map[flow.Identifier]struct{}, len(blockIDs))
	blockHeaders := make([]provider.BlockMetadata, 0, len(blockIDs))

	for _, blockID := range blockIDs {
		// skip duplicates
		if _, exists := seen[blockID]; exists {
			continue
		}
		seen[blockID] = struct{}{}

		header, err := e.headers.ByBlockID(blockID)
		if err != nil {
			return nil, nil, rpc.ConvertStorageError(fmt.Errorf("failed to get block header for %s: %w", blockID, err))
		}

		if header.View >= newestView {
			newestView = header.View
			newestBlockID = blockID
		}

		blockHeaders = append(blockHeaders, provider.BlockMetadata{
			ID:        blockID,
			Height:    header.Height,
			Timestamp: time.UnixMilli(int64(header.Timestamp)).UTC(),
		})
	}

	// get the result for the block with the highest height. all data queried for this set of blocks
	// must be from the execution fork terminating at this result. this guarantees the response
	// contains a consistent view of the state.
	execResultInfo, err := e.execResultProvider.ExecutionResultInfo(
		newestBlockID,
		criteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result for block %v: %w", newestBlockID, err)
	}

	resp, metadata, err := e.provider.Events(
		ctx,
		blockHeaders,
		flow.EventType(eventType),
		requiredEventEncodingVersion,
		execResultInfo,
	)
	if err != nil {
		return nil, metadata, err
	}

	return resp.Events, metadata, nil
}
