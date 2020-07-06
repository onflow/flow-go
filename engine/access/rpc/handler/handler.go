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

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Handler implements the Access API. It is composed of several sub-handlers that implement part of the Access API.
// Script related calls are handled by handlerScripts
// Transaction related calls are handled by handlerTransactions
// Block Header related calls are handled by handlerBlockHeaders
// Block details related calls are handled by handlerBlockDetails
// Event related calls are handled by handlerEvents
// Account related calls are handled by handlerAccounts
// All remaining calls are handled in this file by Handler
type Handler struct {
	handlerScripts
	handlerTransactions
	handlerEvents
	handlerBlockHeaders
	handlerBlockDetails
	handlerAccounts

	executionRPC execution.ExecutionAPIClient
	state        protocol.State
	chainID      flow.ChainID
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
	transactions storage.Transactions,
	chainID flow.ChainID) *Handler {
	return &Handler{
		executionRPC: e,
		state:        s,
		// create the sub-handlers
		handlerScripts: handlerScripts{
			headers:      headers,
			executionRPC: e,
			state:        s,
		},
		handlerTransactions: handlerTransactions{
			collectionRPC: c,
			executionRPC:  e,
			state:         s,
			chainID:       chainID,
			collections:   collections,
			blocks:        blocks,
			transactions:  transactions,
		},
		handlerEvents: handlerEvents{
			executionRPC: e,
			state:        s,
			blocks:       blocks,
		},
		handlerBlockHeaders: handlerBlockHeaders{
			headers: headers,
			state:   s,
		},
		handlerBlockDetails: handlerBlockDetails{
			blocks: blocks,
			state:  s,
		},
		handlerAccounts: handlerAccounts{
			executionRPC: e,
			state:        s,
			chainID:      chainID,
			headers:      headers,
		},
		chainID: chainID,
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

	reqCollID := req.GetId()
	id, err := convert.CollectionID(reqCollID)
	if err != nil {
		return nil, err
	}

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

func (h *Handler) GetNetworkParameters(_ context.Context, _ *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return &access.GetNetworkParametersResponse{
		ChainId: string(h.chainID),
	}, nil
}

func convertStorageError(err error) error {
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}
	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
