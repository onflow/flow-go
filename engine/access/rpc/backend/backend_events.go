package backend

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendEvents struct {
	headers           storage.Headers
	events            storage.Events
	executionReceipts storage.ExecutionReceipts
	state             protocol.State
	connFactory       connection.ConnectionFactory
	log               zerolog.Logger
	maxHeightRange    uint
	nodeCommunicator  Communicator
}

// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and
// the end block height (inclusive) that have the given type.
func (b *backendEvents) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {

	if endHeight < startHeight {
		return nil, status.Error(codes.InvalidArgument, "start height must not be larger than end height")
	}

	rangeSize := endHeight - startHeight + 1 // range is inclusive on both ends
	if rangeSize > uint64(b.maxHeightRange) {
		return nil, status.Errorf(codes.InvalidArgument,
			"requested block range (%d) exceeded maximum (%d)", rangeSize, b.maxHeightRange)
	}

	// get the latest sealed block header
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// sealed block must be in the store, so return an Internal code even if we got NotFound
		return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
	}

	// start height should not be beyond the last sealed height
	if startHeight > sealed.Height {
		return nil, status.Errorf(codes.OutOfRange,
			"start height %d is greater than the last sealed block height %d", startHeight, sealed.Height)
	}

	// limit max height to last sealed block in the chain
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
	blockHeaders := make([]*flow.Header, 0)

	for i := startHeight; i <= endHeight; i++ {
		header, err := b.headers.ByHeight(i)
		if err != nil {
			return nil, rpc.ConvertStorageError(fmt.Errorf("failed to get events: %w", err))
		}

		blockHeaders = append(blockHeaders, header)
	}

	return b.getBlockEvents(ctx, blockHeaders, eventType, requiredEventEncodingVersion)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (b *backendEvents) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {

	if uint(len(blockIDs)) > b.maxHeightRange {
		return nil, status.Errorf(codes.InvalidArgument, "requested block range (%d) exceeded maximum (%d)", len(blockIDs), b.maxHeightRange)
	}

	// find the block headers for all the block IDs
	blockHeaders := make([]*flow.Header, 0)
	for _, blockID := range blockIDs {
		header, err := b.headers.ByBlockID(blockID)
		if err != nil {
			return nil, rpc.ConvertStorageError(fmt.Errorf("failed to get events: %w", err))
		}

		blockHeaders = append(blockHeaders, header)
	}

	return b.getBlockEvents(ctx, blockHeaders, eventType, requiredEventEncodingVersion)
}

// getBlockEvents retrieves events for all the specified blocks that have the given type
// It gets all events available on storage, and requests the rest to an execution node.
func (b *backendEvents) getBlockEvents(
	ctx context.Context,
	blockHeaders []*flow.Header,
	eventType string,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	localResponse, missingHeaders, err := b.getBlockEventsFromStorage(ctx, blockHeaders, eventType, requiredEventEncodingVersion)
	if err != nil {
		return nil, err
	}

	enResponse, err := b.getBlockEventsFromExecutionNode(ctx, missingHeaders, eventType, requiredEventEncodingVersion)
	if err != nil {
		return nil, err
	}

	// sort ascending by block height
	// Note: this may not match the order of the original request for clients using GetEventsForBlockIDs
	// that provide out of order block IDs
	response := append(localResponse, enResponse...)
	sort.Slice(response, func(i, j int) bool {
		return response[i].BlockHeight < response[j].BlockHeight
	})

	return response, nil
}

// getBlockEventsFromStorage retrieves events for all the specified blocks that have the given type
// from the local storage
func (b *backendEvents) getBlockEventsFromStorage(
	ctx context.Context,
	blockHeaders []*flow.Header,
	eventType string,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, []*flow.Header, error) {
	target := flow.EventType(eventType)

	missing := make([]*flow.Header, 0)
	resp := make([]flow.BlockEvents, 0)
	for _, header := range blockHeaders {
		if ctx.Err() != nil {
			return nil, nil, rpc.ConvertError(ctx.Err(), "failed to get events", codes.Canceled)
		}

		events, err := b.events.ByBlockID(header.ID())
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				missing = append(missing, header)
				continue
			}
			return nil, nil, rpc.ConvertError(err, "failed to get events", codes.Internal)
		}

		filteredEvents := make([]flow.Event, 0)
		for _, e := range events {
			if e.Type != target {
				continue
			}

			if requiredEventEncodingVersion == entities.EventEncodingVersion_CCF_V0 {
				payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
				if err != nil {
					return nil, nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
				}
				e.Payload = payload
			}

			filteredEvents = append(filteredEvents, e)
		}

		resp = append(resp, flow.BlockEvents{
			BlockID:        header.ID(),
			BlockHeight:    header.Height,
			BlockTimestamp: header.Timestamp,
			Events:         filteredEvents,
		})
	}

	return resp, missing, nil
}

// getBlockEventsFromExecutionNode retrieves events for all the specified blocks that have the given type
// from an execution node
func (b *backendEvents) getBlockEventsFromExecutionNode(
	ctx context.Context,
	blockHeaders []*flow.Header,
	eventType string,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {

	// create an execution API request for events at block ID
	blockIDs := make([]flow.Identifier, len(blockHeaders))
	for i := range blockIDs {
		blockIDs[i] = blockHeaders[i].ID()
	}

	if len(blockIDs) == 0 {
		return []flow.BlockEvents{}, nil
	}

	req := &execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	// choose the last block ID to find the list of execution nodes
	lastBlockID := blockIDs[len(blockIDs)-1]

	execNodes, err := executionNodesForBlockID(ctx, lastBlockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		b.log.Error().Err(err).Msg("failed to retrieve events from execution node")
		return nil, rpc.ConvertError(err, "failed to retrieve events from execution node", codes.Internal)
	}

	var resp *execproto.GetEventsForBlockIDsResponse
	var successfulNode *flow.Identity
	resp, successfulNode, err = b.getEventsFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		b.log.Error().Err(err).Msg("failed to retrieve events from execution nodes")
		return nil, rpc.ConvertError(err, "failed to retrieve events from execution nodes", codes.Internal)
	}
	b.log.Trace().
		Str("execution_id", successfulNode.String()).
		Str("last_block_id", lastBlockID.String()).
		Msg("successfully got events")

	// convert execution node api result to access node api result
	results, err := verifyAndConvertToAccessEvents(
		resp.GetResults(),
		blockHeaders,
		resp.GetEventEncodingVersion(),
		requiredEventEncodingVersion,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to verify retrieved events from execution node: %v", err)
	}

	return results, nil
}

// verifyAndConvertToAccessEvents converts execution node api result to access node api result, and verifies that the results contains
// results from each block that was requested
func verifyAndConvertToAccessEvents(
	execEvents []*execproto.GetEventsForBlockIDsResponse_Result,
	requestedBlockHeaders []*flow.Header,
	from entities.EventEncodingVersion,
	to entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	if len(execEvents) != len(requestedBlockHeaders) {
		return nil, errors.New("number of results does not match number of blocks requested")
	}

	requestedBlockHeaderSet := map[string]*flow.Header{}
	for _, header := range requestedBlockHeaders {
		requestedBlockHeaderSet[header.ID().String()] = header
	}

	results := make([]flow.BlockEvents, len(execEvents))

	for i, result := range execEvents {
		header, expected := requestedBlockHeaderSet[hex.EncodeToString(result.GetBlockId())]
		if !expected {
			return nil, fmt.Errorf("unexpected blockID from exe node %x", result.GetBlockId())
		}
		if result.GetBlockHeight() != header.Height {
			return nil, fmt.Errorf("unexpected block height %d for block %x from exe node",
				result.GetBlockHeight(),
				result.GetBlockId())
		}

		events, err := convert.MessagesToEventsWithEncodingConversion(result.GetEvents(), from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal events in event %d with encoding version %s: %w",
				i, to.String(), err)
		}

		results[i] = flow.BlockEvents{
			BlockID:        header.ID(),
			BlockHeight:    header.Height,
			BlockTimestamp: header.Timestamp,
			Events:         events,
		}
	}

	return results, nil
}

// getEventsFromAnyExeNode retrieves the given events from any EN in `execNodes`.
// We attempt querying each EN in sequence. If any EN returns a valid response, then errors from
// other ENs are logged and swallowed. If all ENs fail to return a valid response, then an
// error aggregating all failures is returned.
func (b *backendEvents) getEventsFromAnyExeNode(ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, *flow.Identity, error) {
	var resp *execproto.GetEventsForBlockIDsResponse
	var execNode *flow.Identity
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			start := time.Now()
			resp, err = b.tryGetEvents(ctx, node, req)
			duration := time.Since(start)

			logger := b.log.With().
				Str("execution_node", node.String()).
				Str("event", req.GetType()).
				Int("blocks", len(req.BlockIds)).
				Int64("rtt_ms", duration.Milliseconds()).
				Logger()

			if err == nil {
				// return if any execution node replied successfully
				logger.Debug().Msg("Successfully got events")
				execNode = node
				return nil
			}

			logger.Err(err).Msg("failed to execute GetEvents")
			return err
		},
		nil,
	)

	return resp, execNode, errToReturn
}

func (b *backendEvents) tryGetEvents(ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return execRPCClient.GetEventsForBlockIDs(ctx, req)
}
