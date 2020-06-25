package handler

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type handlerAccounts struct {
	state        protocol.State
	executionRPC execution.ExecutionAPIClient
	headers      storage.Headers
}

func (h *handlerAccounts) GetAccount(ctx context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {

	address := req.GetAddress()

	if address == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	// get the latest sealed header
	latestHeader, err := h.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	latestBlockID := latestHeader.ID()

	account, err := h.getAccountAtBlockID(ctx, address, latestBlockID)

	if err != nil {
		return nil, err
	}

	return &access.GetAccountResponse{
		Account: account,
	}, nil

}

func (h *handlerAccounts) getAccountAtBlockID(ctx context.Context, address []byte, blockID flow.Identifier) (*entities.Account, error) {

	exeReq := execution.GetAccountAtBlockIDRequest{
		Address: address,
		BlockId: blockID[:],
	}

	exeResp, err := h.executionRPC.GetAccountAtBlockID(ctx, &exeReq)
	if err != nil {
		errStatus, _ := status.FromError(err)
		if errStatus.Code() == codes.NotFound {
			return nil, err
		}

		return nil, status.Errorf(codes.Internal, "failed to get account from the execution node: %v", err)
	}
	return exeResp.GetAccount(), nil
}
