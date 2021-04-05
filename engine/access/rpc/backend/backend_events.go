package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendEvents struct {
	staticExecutionRPC execproto.ExecutionAPIClient
	headers            storage.Headers
	executionReceipts  storage.ExecutionReceipts
	state              protocol.State
	connFactory        ConnectionFactory
	log                zerolog.Logger
	maxHeightRange     uint
}

// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and
// the end block height (inclusive) that have the given type.
func (b *backendEvents) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
) ([]flow.BlockEvents, error) {

	if endHeight < startHeight {
		return nil, status.Error(codes.InvalidArgument, "invalid start or end height")
	}

	// get the latest sealed block header
	head, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, " failed to get events: %v", err)
	}

	// start height should not be beyond the last sealed height
	if head.Height < startHeight {
		return nil, status.Errorf(codes.Internal,
			" start height %d is greater than the last sealed block height %d", startHeight, head.Height)
	}

	// limit max height to last sealed block in the chain
	if head.Height < endHeight {
		endHeight = head.Height
	}

	// find the block headers for all the blocks between min and max height (inclusive)
	blockIDs := make([]flow.Identifier, 0, endHeight-startHeight+1)

	for i := startHeight; i <= endHeight; i++ {
		blockID, err := b.headers.IDByHeight(i)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}

		blockIDs = append(blockIDs, blockID)
	}

	return b.getBlockEventsFromExecutionNode(ctx, blockIDs, eventType)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (b *backendEvents) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
) ([]flow.BlockEvents, error) {

	// forward the request to the execution node
	return b.getBlockEventsFromExecutionNode(ctx, blockIDs, eventType)
}

func (b *backendEvents) getBlockEventsFromExecutionNode(
	ctx context.Context,
	blockIDs []flow.Identifier,
	eventType string,
) ([]flow.BlockEvents, error) {

	if len(blockIDs) == 0 {
		return []flow.BlockEvents{}, nil
	}

	// limit height range queries
	if uint(len(blockIDs)) > b.maxHeightRange {
		return nil, fmt.Errorf("requested block range (%d) exceeded maximum (%d)", len(blockIDs), b.maxHeightRange)
	}

	req := execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	// choose the last block ID to find the list of execution nodes
	lastBlockID := blockIDs[len(blockIDs)-1]

	execNodes, err := executionNodesForBlockID(lastBlockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
	}

	var resp *execproto.GetEventsForBlockIDsResponse
	if len(execNodes) == 0 {
		if b.staticExecutionRPC == nil {
			return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node")
		}

		// call the execution node gRPC
		resp, err = b.staticExecutionRPC.GetEventsForBlockIDs(ctx, &req)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
		}
	} else {
		var successfulNode *flow.Identity
		resp, successfulNode, err = b.getEventsFromAnyExeNode(ctx, execNodes, req)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution nodes %s: %v", execNodes, err)
		}
		b.log.Trace().
			Str("execution_id", successfulNode.String()).
			Str("last_block_id", lastBlockID.String()).
			Msg("successfully got events")
	}

	// convert execution node api result to access node api result
	results, err := verifyAndConvertToAccessEvents(resp.GetResults(), blockIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to verify retrieved events from execution node: %v", err)
	}

	return results, nil
}

// verifyAndConvertToAccessEvents converts execution node api result to access node api result, and verifies that the results contains
// results from each block that was requested
func verifyAndConvertToAccessEvents(execEvents []*execproto.GetEventsForBlockIDsResponse_Result, requestedBlockIDs []flow.Identifier) ([]flow.BlockEvents, error) {
	if len(execEvents) != len(requestedBlockIDs) {
		return nil, errors.New("number of results does not match number of blocks requested")
	}

	// ensure that each block we requested appears exactly once
	// TODO we should remove from the set in the loop, and check that the set has len 0 after the loop
	requestedBlockHeaderSet := map[flow.Identifier]struct{}{}
	for _, blockID := range requestedBlockIDs {
		requestedBlockHeaderSet[blockID] = struct{}{}
	}

	results := make([]flow.BlockEvents, len(execEvents))

	for i, result := range execEvents {
		blockID := flow.BytesToIdentifier(result.GetBlockId())
		_, expected := requestedBlockHeaderSet[blockID]
		if !expected {
			return nil, fmt.Errorf("unexpected blockID from exe node %x", result.GetBlockId())
		}

		results[i] = flow.BlockEvents{
			BlockID:     blockID,
			BlockHeight: result.GetBlockHeight(),
			// TODO omitted to avoid needing to look up entire header
			//BlockTimestamp: header.Timestamp,
			Events: convert.MessagesToEvents(result.GetEvents()),
		}
	}

	return results, nil
}

func (b *backendEvents) getEventsFromAnyExeNode(ctx context.Context,
	execNodes flow.IdentityList,
	req execproto.GetEventsForBlockIDsRequest) (*execproto.GetEventsForBlockIDsResponse, *flow.Identity, error) {
	var errors *multierror.Error
	// try to get events from one of the execution nodes
	for _, execNode := range execNodes {
		resp, err := b.tryGetEvents(ctx, execNode, req)
		if err == nil {
			return resp, execNode, nil
		}
		errors = multierror.Append(errors, err)
	}
	return nil, nil, errors.ErrorOrNil()
}

func (b *backendEvents) tryGetEvents(ctx context.Context,
	execNode *flow.Identity,
	req execproto.GetEventsForBlockIDsRequest) (*execproto.GetEventsForBlockIDsResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	resp, err := execRPCClient.GetEventsForBlockIDs(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
