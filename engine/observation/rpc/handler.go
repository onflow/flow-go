package rpc

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"

	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Handler implements a subset of the Access API. It spans multiple files
// Transaction related calls are handled in handler handler_transaction
// Block Header related calls are handled in handler handler_block_header
// Block details related calls are handled in handler handler_block_details
// All remaining calls are handled in this file (or not implemented yet)
type Handler struct {
	access.UnimplementedAccessAPIServer
	executionRPC  access.AccessAPIClient
	collectionRPC access.AccessAPIClient
	log           zerolog.Logger
	state         protocol.State

	// storage
	blocks       storage.Blocks
	headers      storage.Headers
	collections  storage.Collections
	transactions storage.Transactions
}

func NewHandler(log zerolog.Logger,
	s protocol.State,
	e access.AccessAPIClient,
	c access.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions) *Handler {
	return &Handler{
		executionRPC:                 e,
		collectionRPC:                c,
		blocks:                       blocks,
		headers:                      headers,
		collections:                  collections,
		transactions:                 transactions,
		state:                        s,
		log:                          log,
		UnimplementedAccessAPIServer: access.UnimplementedAccessAPIServer{},
	}
}

// Ping responds to requests when the server is up.
func (h *Handler) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

func (h *Handler) ExecuteScriptAtLatestBlock(ctx context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	return h.executionRPC.ExecuteScriptAtLatestBlock(ctx, req)
}

func (h *Handler) getLatestSealedHeader() (*flow.Header, error) {
	// lookup the latest seal to get latest blockid
	seal, err := h.state.Final().Seal()
	if err != nil {
		return nil, err
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

func convertStorageError(err error) error {
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}
	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
