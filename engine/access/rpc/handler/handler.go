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

// Handler implements the Access API. It spans multiple files
// Transaction related calls are handled in handler handler_transaction
// Block Header related calls are handled in handler handler_block_header
// Block details related calls are handled in handler handler_block_details
// All remaining calls are handled in this file
type Handler struct {
	executionRPC  execution.ExecutionAPIClient
	collectionRPC access.AccessAPIClient
	log           zerolog.Logger
	state         protocol.State

	// storage
	blocks       storage.Blocks
	headers      storage.Headers
	collections  storage.Collections
	transactions storage.Transactions
}

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
		executionRPC:  e,
		collectionRPC: c,
		blocks:        blocks,
		headers:       headers,
		collections:   collections,
		transactions:  transactions,
		state:         s,
		log:           log,
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

func (h *Handler) getLatestSealedHeader() (*flow.Header, error) {
	// lookup the latest seal to get latest blockid
	seal, err := h.state.Final().Seal()
	if err != nil {
		return nil, fmt.Errorf("could not get latest sealed block ID: %w", err)
	}

	if seal.BlockID == flow.ZeroID {
		// TODO: Figure out how to handle the very first seal, for now, just using latest finalized for script
		return h.state.Final().Head()
	}
	// query header storage for that blockid
	return h.headers.ByBlockID(seal.BlockID)
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
	latestHeader, err := h.getLatestSealedHeader()
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

func convertStorageError(err error) error {
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}
	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
