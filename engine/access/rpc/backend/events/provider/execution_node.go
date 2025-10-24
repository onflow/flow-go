package provider

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// ENEventProvider retrieves events by querying remote Execution Nodes (ENs).
// It selects from available executors for a given execution result and
// aggregates responses, converting them into the Access API format.
type ENEventProvider struct {
	log              zerolog.Logger
	nodeProvider     *rpc.ExecutionNodeIdentitiesProvider
	connFactory      connection.ConnectionFactory
	nodeCommunicator node_communicator.Communicator
}

var _ EventProvider = (*ENEventProvider)(nil)

func NewENEventProvider(
	log zerolog.Logger,
	nodeProvider *rpc.ExecutionNodeIdentitiesProvider,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
) *ENEventProvider {
	return &ENEventProvider{
		log:              log.With().Str("event_provider", "execution_node").Logger(),
		nodeProvider:     nodeProvider,
		connFactory:      connFactory,
		nodeCommunicator: nodeCommunicator,
	}
}

func (e *ENEventProvider) Events(
	ctx context.Context,
	blocks []BlockMetadata,
	eventType flow.EventType,
	encodingVersion entities.EventEncodingVersion,
	execResultInfo *optimistic_sync.ExecutionResultInfo,
) (Response, *access.ExecutorMetadata, error) {
	if len(blocks) == 0 {
		return Response{}, nil, nil
	}

	blockIDs := make([]flow.Identifier, len(blocks))
	for i := range blocks {
		blockIDs[i] = blocks[i].ID
	}

	req := &execproto.GetEventsForBlockIDsRequest{
		Type:     string(eventType),
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	resp, node, err := e.getEventsFromAnyExeNode(ctx, execResultInfo.ExecutionNodes, req)
	if err != nil {
		return Response{}, nil,
			rpc.ConvertError(err, "failed to get execution nodes for events query", codes.Internal)
	}

	// convert execution node api result to access node api result
	results, err := verifyAndConvertToAccessEvents(
		resp.GetResults(),
		blocks,
		resp.GetEventEncodingVersion(),
		encodingVersion,
	)
	if err != nil {
		return Response{}, nil,
			status.Errorf(codes.Internal, "failed to verify retrieved events from execution node: %v", err)
	}

	metadata := &access.ExecutorMetadata{
		ExecutionResultID: execResultInfo.ExecutionResultID,
		ExecutorIDs:       orderedExecutors(node.NodeID, execResultInfo.ExecutionNodes.NodeIDs()),
	}

	return Response{
		Events: results,
	}, metadata, nil
}

// getEventsFromAnyExeNode retrieves the given events from any EN in `execNodes`.
// We attempt querying each EN in sequence. If any EN returns a valid response, then errors from
// other ENs are logged and swallowed. If all ENs fail to return a valid response, then an
// error aggregating all failures is returned.
func (e *ENEventProvider) getEventsFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, *flow.IdentitySkeleton, error) {
	var resp *execproto.GetEventsForBlockIDsResponse
	var execNode *flow.IdentitySkeleton
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			start := time.Now()
			resp, err = e.tryGetEvents(ctx, node, req)
			duration := time.Since(start)

			logger := e.log.With().
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

			logger.Err(err).Msg("failed to execute Events")
			return err
		},
		nil,
	)

	return resp, execNode, errToReturn
}

func (e *ENEventProvider) tryGetEvents(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return execRPCClient.GetEventsForBlockIDs(ctx, req)
}

// verifyAndConvertToAccessEvents converts execution node api result to access node api result,
// and verifies that the results contains results from each block that was requested
func verifyAndConvertToAccessEvents(
	execEvents []*execproto.GetEventsForBlockIDsResponse_Result,
	requestedBlockInfos []BlockMetadata,
	from entities.EventEncodingVersion,
	to entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	if len(execEvents) != len(requestedBlockInfos) {
		return nil, errors.New("number of results does not match number of blocks requested")
	}

	requestedBlockInfoSet := map[string]BlockMetadata{}
	for _, header := range requestedBlockInfos {
		requestedBlockInfoSet[header.ID.String()] = header
	}

	results := make([]flow.BlockEvents, len(execEvents))

	for i, result := range execEvents {
		blockInfo, expected := requestedBlockInfoSet[hex.EncodeToString(result.GetBlockId())]
		if !expected {
			return nil, fmt.Errorf("unexpected blockID from exe node %x", result.GetBlockId())
		}
		if result.GetBlockHeight() != blockInfo.Height {
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
			BlockID:        blockInfo.ID,
			BlockHeight:    blockInfo.Height,
			BlockTimestamp: blockInfo.Timestamp,
			Events:         events,
		}
	}

	return results, nil
}

// orderedExecutors creates an ordered list of executors for the same execution result
// - respondingExecutor is the executor who returned an execution result.
// - executorList is the full list of executors who produced the same execution result.
func orderedExecutors(respondingExecutor flow.Identifier, executorList flow.IdentifierList) flow.IdentifierList {
	ordered := make(flow.IdentifierList, 0, len(executorList))
	ordered = append(ordered, respondingExecutor)

	for _, nodeID := range executorList {
		if nodeID != respondingExecutor {
			ordered = append(ordered, nodeID)
		}
	}

	return ordered
}
