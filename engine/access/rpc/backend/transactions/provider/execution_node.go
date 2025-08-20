package provider

import (
	"context"
	"errors"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ENTransactionProvider struct {
	log   zerolog.Logger
	state protocol.State

	collections storage.Collections

	connFactory      connection.ConnectionFactory
	nodeCommunicator node_communicator.Communicator
	nodeProvider     *rpc.ExecutionNodeIdentitiesProvider

	txStatusDeriver *txstatus.TxStatusDeriver

	systemTxID flow.Identifier
	systemTx   *flow.TransactionBody
}

var _ TransactionProvider = (*ENTransactionProvider)(nil)

func NewENTransactionProvider(
	log zerolog.Logger,
	state protocol.State,
	collections storage.Collections,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
	txStatusDeriver *txstatus.TxStatusDeriver,
	systemTxID flow.Identifier,
	systemTx *flow.TransactionBody,
) *ENTransactionProvider {

	return &ENTransactionProvider{
		log:              log.With().Str("transaction_provider", "execution_node").Logger(),
		state:            state,
		collections:      collections,
		connFactory:      connFactory,
		nodeCommunicator: nodeCommunicator,
		nodeProvider:     execNodeIdentitiesProvider,
		txStatusDeriver:  txStatusDeriver,
		systemTxID:       systemTxID,
		systemTx:         systemTx,
	}
}

func (e *ENTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	_ entities.ExecutionStateQuery,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	// create an execution API request for events at blockID and transactionID
	req := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID[:],
	}

	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		// if no execution receipt were found, return a NotFound GRPC error
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, err
	}

	resp, err := e.getTransactionResultFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, err
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to convert events to message", codes.Internal)
	}

	return &accessmodel.TransactionResult{
		TransactionID: transactionID,
		Status:        txStatus,
		StatusCode:    uint(resp.GetStatusCode()),
		Events:        events,
		ErrorMessage:  resp.GetErrorMessage(),
		BlockID:       blockID,
		BlockHeight:   block.Height,
	}, nil
}

func (e *ENTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
	_ entities.ExecutionStateQuery,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	// create request and forward to EN
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
	}

	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	resp, err := e.getTransactionResultByIndexFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Header.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert events in blockID %x: %v", blockID, err)
	}

	// convert to response, cache and return
	return &accessmodel.TransactionResult{
		Status:       txStatus,
		StatusCode:   uint(resp.GetStatusCode()),
		Events:       events,
		ErrorMessage: resp.GetErrorMessage(),
		BlockID:      blockID,
		BlockHeight:  block.Header.Height,
	}, nil
}

func (e *ENTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	_ entities.ExecutionStateQuery,
) ([]*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}

	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	resp, err := e.getTransactionResultsByBlockIDFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	results := make([]*accessmodel.TransactionResult, 0, len(resp.TransactionResults))
	i := 0
	errInsufficientResults := status.Errorf(
		codes.Internal,
		"number of transaction results returned by execution node is less than the number of transactions in the block",
	)

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		for _, txID := range collection.Transactions {
			// bounds check. this means the EN returned fewer transaction results than the transactions  in the block
			if i >= len(resp.TransactionResults) {
				return nil, errInsufficientResults
			}
			txResult := resp.TransactionResults[i]

			// tx body is irrelevant to status if it's in an executed block
			txStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Header.Height, true)
			if err != nil {
				if !errors.Is(err, state.ErrUnknownSnapshotReference) {
					irrecoverable.Throw(ctx, err)
				}
				return nil, rpc.ConvertStorageError(err)
			}
			events, err := convert.MessagesToEventsWithEncodingConversion(txResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
			if err != nil {
				return nil, status.Errorf(codes.Internal,
					"failed to convert events to message in txID %x: %v", txID, err)
			}

			results = append(results, &accessmodel.TransactionResult{
				Status:        txStatus,
				StatusCode:    uint(txResult.GetStatusCode()),
				Events:        events,
				ErrorMessage:  txResult.GetErrorMessage(),
				BlockID:       blockID,
				TransactionID: txID,
				CollectionID:  guarantee.CollectionID,
				BlockHeight:   block.Header.Height,
			})

			i++
		}
	}

	// after iterating through all transactions  in each collection, i equals the total number of
	// user transactions  in the block
	txCount := i
	sporkRootBlockHeight := e.state.Params().SporkRootBlockHeight()

	// root block has no system transaction result
	if block.Header.Height > sporkRootBlockHeight {
		// system chunk transaction

		// resp.TransactionResultsByBlockID includes the system tx result, so there should be exactly one
		// more result than txCount
		if txCount != len(resp.TransactionResults)-1 {
			if txCount >= len(resp.TransactionResults) {
				return nil, errInsufficientResults
			}
			// otherwise there are extra results
			// TODO(bft): slashable offense
			return nil, status.Errorf(codes.Internal, "number of transaction results returned by execution node is more than the number of transactions  in the block")
		}

		systemTxResult := resp.TransactionResults[len(resp.TransactionResults)-1]
		systemTxStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Header.Height, true)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert events from system tx result", codes.Internal)
		}

		results = append(results, &accessmodel.TransactionResult{
			Status:        systemTxStatus,
			StatusCode:    uint(systemTxResult.GetStatusCode()),
			Events:        events,
			ErrorMessage:  systemTxResult.GetErrorMessage(),
			BlockID:       blockID,
			TransactionID: e.systemTxID,
			BlockHeight:   block.Header.Height,
		})
	}
	return results, nil
}

func (e *ENTransactionProvider) getTransactionResultFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	var errToReturn error

	defer func() {
		if errToReturn != nil {
			e.log.Info().Err(errToReturn).Msg("failed to get transaction result from execution nodes")
		}
	}()

	var resp *execproto.GetTransactionResultResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResult(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Hex("transaction_id", req.GetTransactionId()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (e *ENTransactionProvider) getTransactionResultsByBlockIDFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	var errToReturn error

	defer func() {
		// log the errors
		if errToReturn != nil {
			e.log.Err(errToReturn).Msg("failed to get transaction results from execution nodes")
		}
	}()

	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionResultsResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResultsByBlockID(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (e *ENTransactionProvider) getTransactionResultByIndexFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	var errToReturn error
	defer func() {
		if errToReturn != nil {
			e.log.Info().Err(errToReturn).Msg("failed to get transaction result from execution nodes")
		}
	}()

	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes provided")
	}

	var resp *execproto.GetTransactionResultResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResultByIndex(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Uint32("index", req.GetIndex()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (e *ENTransactionProvider) tryGetTransactionResult(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResult(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (e *ENTransactionProvider) tryGetTransactionResultsByBlockID(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultsByBlockID(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (e *ENTransactionProvider) tryGetTransactionResultByIndex(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultByIndex(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
