package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

// ENAccountProvider is a provider that reads account data using execution nodes.
type ENAccountProvider struct {
	log                        zerolog.Logger
	state                      protocol.State
	connFactory                connection.ConnectionFactory
	nodeCommunicator           node_communicator.Communicator
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
}

var _ AccountProvider = (*ENAccountProvider)(nil)

// NewENAccountProvider creates a new instance of ENAccountProvider.
func NewENAccountProvider(
	log zerolog.Logger,
	state protocol.State,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	execNodeIdentityProvider *rpc.ExecutionNodeIdentitiesProvider,
) *ENAccountProvider {
	return &ENAccountProvider{
		log:                        log.With().Str("account_provider", "execution_node").Logger(),
		state:                      state,
		connFactory:                connFactory,
		nodeCommunicator:           nodeCommunicator,
		execNodeIdentitiesProvider: execNodeIdentityProvider,
	}
}

// GetAccountAtBlock returns a Flow account for the given address and block ID.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
//   - [access.DataNotFoundError]: If the requested data is not found.
//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
//   - [access.RequestCanceledError]: If the request is canceled.
//   - [access.RequestTimedOutError]: If the request times out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [access.InternalError]: For internal execution node failures.
func (e *ENAccountProvider) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	_ uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	req := &execproto.GetAccountAtBlockIDRequest{
		Address: address.Bytes(),
		BlockId: blockID[:],
	}

	var resp *execproto.GetAccountAtBlockIDResponse
	var nodeID flow.Identifier
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		executionResultInfo.ExecutionNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			start := time.Now()
			nodeID = node.NodeID

			resp, err = e.tryGetAccount(ctx, node.Address, req)
			duration := time.Since(start)

			lg := e.log.With().
				Str("execution_node", node.String()).
				Hex("block_id", req.GetBlockId()).
				Hex("address", req.GetAddress()).
				Int64("rtt_ms", duration.Milliseconds()).
				Logger()

			if err != nil {
				lg.Err(err).Msg("Failed to execute GetAccount")
				return err
			}

			// return if any execution node replies successfully
			lg.Debug().Msg("Successfully retrieved account info")
			return nil
		},
		func(node *flow.IdentitySkeleton, err error) bool {
			return status.Code(err) == codes.InvalidArgument
		},
	)

	if errToReturn != nil {
		switch {
		case errors.Is(errToReturn, context.Canceled):
			return nil, nil, access.NewRequestCanceledError(errToReturn)
		case errors.Is(errToReturn, context.DeadlineExceeded):
			return nil, nil, access.NewRequestTimedOutError(errToReturn)
		}

		switch status.Code(errToReturn) {
		case codes.InvalidArgument:
			return nil, nil, access.NewInvalidRequestError(errToReturn)
		case codes.NotFound:
			return nil, nil, access.NewDataNotFoundError("block", errToReturn)
		case codes.OutOfRange:
			return nil, nil, access.NewOutOfRangeError(errToReturn)
		case codes.Internal:
			return nil, nil, access.NewInternalError(errToReturn)
		case codes.Unavailable:
			return nil, nil, access.NewServiceUnavailable(errToReturn)
		default:
			return nil, nil, access.NewInternalError(errToReturn)
		}
	}

	account, err := convert.MessageToAccount(resp.GetAccount())
	if err != nil {
		err = fmt.Errorf("failed to convert account message: %w", err)
		return nil, nil, access.NewInternalError(err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       common.OrderedExecutors(nodeID, executionResultInfo.ExecutionNodes.NodeIDs()),
	}

	return account, metadata, nil
}

// GetAccountBalanceAtBlock returns the balance of a Flow account
// at the given block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
//   - [access.DataNotFoundError]: If the requested data is not found.
//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
//   - [access.RequestCanceledError]: If the request is canceled.
//   - [access.RequestTimedOutError]: If the request times out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [access.InternalError]: For internal execution node failures.
func (e *ENAccountProvider) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (uint64, *accessmodel.ExecutorMetadata, error) {
	account, metadata, err := e.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		return 0, nil, err
	}

	return account.Balance, metadata, nil
}

// GetAccountKeyAtBlock returns a specific public key of a Flow account
// by its key index and block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
//   - [access.DataNotFoundError]: If the requested data is not found.
//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
//   - [access.RequestCanceledError]: If the request is canceled.
//   - [access.RequestTimedOutError]: If the request times out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [access.InternalError]: For internal execution node failures.
func (e *ENAccountProvider) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	account, metadata, err := e.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		return nil, nil, err
	}

	for _, key := range account.Keys {
		if key.Index == keyIndex {
			return &key, metadata, nil
		}
	}

	err = fmt.Errorf("failed to get account key by index: %d", keyIndex)
	return nil, nil, access.NewDataNotFoundError("key", err)
}

// GetAccountKeysAtBlock returns all public keys associated with a Flow account
// at the given block height.
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
//   - [access.DataNotFoundError]: If the requested data is not found.
//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
//   - [access.RequestCanceledError]: If the request is canceled.
//   - [access.RequestTimedOutError]: If the request times out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [access.InternalError]: For internal execution node failures.
func (e *ENAccountProvider) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	account, metadata, err := e.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		return nil, nil, err
	}

	return account.Keys, metadata, nil
}

// tryGetAccount attempts to get the account from the given execution node.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable]: If a connection to an execution node cannot be established.
//   - [status.Error]: If the execution node returns a gRPC error.
func (e *ENAccountProvider) tryGetAccount(
	ctx context.Context,
	executorAddress string,
	req *execproto.GetAccountAtBlockIDRequest,
) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to execution node %s: %v", executorAddress, err)
	}
	defer closer.Close()

	return execRPCClient.GetAccountAtBlockID(ctx, req)
}
