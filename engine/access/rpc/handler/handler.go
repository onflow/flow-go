package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Handler implements the Access API. It is composed of several sub-handlers that implement part of the Access API.
// Script related calls are handled by handlerScript
// Transaction related calls are handled by handlerTransactions
// Block Header related calls are handled by handlerBlockHeader
// Block details related calls are handled by handlerBlockDetails
// Event related calls are handled by handlerEvents
// All remaining calls are handled in this file by Handler
type Handler struct {
	handlerScript
	handlerTransaction
	handlerEvents
	handlerBlockHeader
	handlerBlockDetails

	executionRPC execution.ExecutionAPIClient
	state        protocol.State
}

// compile time check to make sure the aggregated handler implements the Access API
var _ access.AccessAPIServer = &Handler{}

func NewHandler(log zerolog.Logger,
	s protocol.State,
	e execution.ExecutionAPIClient,
	c access.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions) *Handler {

	return &Handler{
		executionRPC: e,
		state:        s,
		// create the sub-handlers
		handlerScript: handlerScript{
			headers:      headers,
			executionRPC: e,
			state:        s,
		},
		handlerTransaction: handlerTransaction{
			collectionRPC: c,
			executionRPC:  e,
			state:         s,
			collections:   collections,
			blocks:        blocks,
			transactions:  transactions,
		},
		handlerEvents: handlerEvents{
			executionRPC: e,
			state:        s,
			blocks:       blocks,
		},
		handlerBlockHeader: handlerBlockHeader{
			headers: headers,
			state:   s,
		},
		handlerBlockDetails: handlerBlockDetails{
			blocks: blocks,
			state:  s,
		},
	}
}

// Ping responds to requests when the server is up.
func (h *Handler) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	_, err := h.executionRPC.Ping(ctx, &execution.PingRequest{})
	if err != nil {
		return nil, fmt.Errorf("could not ping execution node: %w", err)
	}
	_, err = h.collectionRPC.Ping(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("could not ping collection node: %w", err)
	}
	return &access.PingResponse{}, nil
}

func (h *Handler) GetCollectionByID(_ context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {

	id := flow.HashToID(req.Id)

	// retrieve the collection from the collection storage
	cl, err := h.collections.LightByID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	transactions := make([]*flow.TransactionBody, len(cl.Transactions))

	// retrieve all transactions from the transaction storage
	for i, txID := range cl.Transactions {
		tx, err := h.transactions.ByID(txID)
		if err != nil {
			err = convertStorageError(err)
			return nil, err
		}
		transactions[i] = tx
	}

	// create a flow collection object
	collection := &flow.Collection{Transactions: transactions}

	// convert flow collection object to protobuf entity
	ce, err := convert.CollectionToMessage(collection)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	// return the collection entity
	resp := &access.CollectionResponse{
		Collection: ce,
	}
	return resp, nil
}

func (h *Handler) GetAccount(ctx context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {

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

	exeReq := execution.GetAccountAtBlockIDRequest{
		Address: address,
		BlockId: latestBlockID[:],
	}

	exeResp, err := h.executionRPC.GetAccountAtBlockID(ctx, &exeReq)
	if err != nil {
		errStatus, _ := status.FromError(err)
		if errStatus.Code() == codes.NotFound {
			return nil, err
		}

		return nil, status.Errorf(codes.Internal, "failed to get account from the execution node: %v", err)
	}

	return &access.GetAccountResponse{
		Account: exeResp.GetAccount(),
	}, nil

}

func (h *Handler) GetNetworkParameters(_ context.Context, _ *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return &access.GetNetworkParametersResponse{
		ChainId: string(flow.GetChainID()),
	}, nil
}

func convertStorageError(err error) error {
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}
	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
