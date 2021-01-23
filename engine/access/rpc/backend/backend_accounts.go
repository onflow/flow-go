package backend

import (
	"context"

	"github.com/hashicorp/go-multierror"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendAccounts struct {
	state              protocol.State
	staticExecutionRPC execproto.ExecutionAPIClient
	headers            storage.Headers
	executionReceipts  storage.ExecutionReceipts
	connFactory        ConnectionFactory
}

func (b *backendAccounts) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return b.GetAccountAtLatestBlock(ctx, address)
}

func (b *backendAccounts) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {

	// get the latest sealed header
	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	latestBlockID := latestHeader.ID()

	account, err := b.getAccountAtBlockID(ctx, address, latestBlockID)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (b *backendAccounts) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (*flow.Account, error) {
	// get header at given height
	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	// get block ID of the header at the given height
	blockID := header.ID()

	account, err := b.getAccountAtBlockID(ctx, address, blockID)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (b *backendAccounts) getAccountAtBlockID(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
) (*flow.Account, error) {

	exeReq := execproto.GetAccountAtBlockIDRequest{
		Address: address.Bytes(),
		BlockId: blockID[:],
	}

	var exeRes *execproto.GetAccountAtBlockIDResponse
	var err error

	if b.staticExecutionRPC != nil {

		exeRes, err = b.staticExecutionRPC.GetAccountAtBlockID(ctx, &exeReq)
		if err != nil {
			convertedErr := getAccountError(err)
			return nil, convertedErr
		}
	} else {

		execNodes, err := executionNodesForBlockID(blockID, b.executionReceipts, b.state)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get account from the execution node: %v", err)
		}

		exeRes, err = b.getAccountFromAnyExeNode(ctx, execNodes, exeReq)
		if err != nil {
			return nil, err
		}
	}

	account, err := convert.MessageToAccount(exeRes.GetAccount())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert account message: %v", err)
	}

	return account, nil
}

func getAccountError(err error) error {
	errStatus, _ := status.FromError(err)
	if errStatus.Code() == codes.NotFound {
		return err
	}
	return status.Errorf(codes.Internal, "failed to get account from the execution node: %v", err)
}

func (b *backendAccounts) getAccountFromAnyExeNode(ctx context.Context, execNodes flow.IdentityList, req execproto.GetAccountAtBlockIDRequest) (*execproto.GetAccountAtBlockIDResponse, error) {
	var errors *multierror.Error // captures all error except
	for _, execNode := range execNodes {
		resp, err := b.tryGetAccount(ctx, execNode, req)
		if err == nil {
			// return if any execution node replied successfully
			return resp, nil
		}
		errors = multierror.Append(errors, err)
	}
	// if we made it till here means there was at least one error
	errToReturn := errors.ErrorOrNil()

	// if there were an any errors other than codes.NotFound, return those
	for _, err := range errors.Errors {
		errStatus, _ := status.FromError(err)
		if errStatus.Code() != codes.NotFound {
			return nil, status.Errorf(codes.Internal, "failed to get account from the execution node: %v", errToReturn)
		}
	}

	// if all errors were codes.NotFound, then return a codes.NotFound error wrapping all those error
	return nil, status.Errorf(codes.NotFound, "failed to get account from the execution node: %v", errToReturn)
}

func (b *backendAccounts) tryGetAccount(ctx context.Context, execNode *flow.Identity, req execproto.GetAccountAtBlockIDRequest) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	resp, err := execRPCClient.GetAccountAtBlockID(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
