package rpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"

	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Handler implements a subset of the Observation API
type Handler struct {
	observation.UnimplementedObserveServiceServer
	executionRPC  observation.ObserveServiceClient
	collectionRPC observation.ObserveServiceClient
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
	e observation.ObserveServiceClient,
	c observation.ObserveServiceClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions) *Handler {
	return &Handler{
		executionRPC:                      e,
		collectionRPC:                     c,
		blocks:                            blocks,
		headers:                           headers,
		collections:                       collections,
		transactions:                      transactions,
		state:                             s,
		log:                               log,
		UnimplementedObserveServiceServer: observation.UnimplementedObserveServiceServer{},
	}
}

// Ping responds to requests when the server is up.
func (h *Handler) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	return &observation.PingResponse{}, nil
}

func (h *Handler) ExecuteScript(ctx context.Context, req *observation.ExecuteScriptRequest) (*observation.ExecuteScriptResponse, error) {
	return h.executionRPC.ExecuteScript(ctx, req)
}

// SendTransaction forwards the transaction to the collection node
func (h *Handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {

	// send the transaction to the collection node
	resp, err := h.collectionRPC.SendTransaction(ctx, req)
	if err != nil {
		return resp, err
	}

	// convert the request message to a transaction
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	// store the transaction locally
	err = h.transactions.Store(&tx)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
	}

	return resp, nil
}

func (h *Handler) GetLatestBlockHeader(ctx context.Context, req *observation.GetLatestBlockHeaderRequest) (*observation.BlockHeaderResponse, error) {

	var header *flow.Header
	var err error
	if req.IsSealed {
		// get the latest seal header from storage
		header, err = h.getLatestSealedHeader()
	} else {
		// get the finalized header from state
		header, err = h.state.Final().Head()
	}

	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	resp := &observation.BlockHeaderResponse{
		Block: &msg,
	}
	return resp, nil
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

func (h *Handler) GetTransaction(_ context.Context, req *observation.GetTransactionRequest) (*observation.TransactionResponse, error) {

	id := flow.HashToID(req.Id)
	// look up transaction from storage
	tx, err := h.transactions.ByID(id)
	if err != nil {
		return nil, getError(err, "transaction")
	}

	// derive status of the transaction
	status, err := h.deriveTransactionStatus(tx)
	if err != nil {
		return nil, getError(err, "transaction")
	}

	// convert flow transaction to a protobuf message
	transaction := convert.TransactionToMessage(*tx)

	// set the status
	transaction.Status = status

	// return result
	resp := &observation.TransactionResponse{
		Transaction: transaction,
	}
	return resp, nil
}

func (h *Handler) GetBlockHeaderByID(_ context.Context, req *observation.GetBlockHeaderByIDRequest) (*observation.BlockHeaderResponse, error) {

	id := flow.HashToID(req.Id)
	header, err := h.headers.ByBlockID(id)
	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not convert block header: %s", err.Error())
	}

	resp := &observation.BlockHeaderResponse{
		Block: &msg,
	}
	return resp, nil
}

func (h *Handler) GetBlockHeaderByHeight(_ context.Context, req *observation.GetBlockHeaderByHeightRequest) (*observation.BlockHeaderResponse, error) {

	header, err := h.headers.ByNumber(req.Height)
	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not convert block header: %s", err.Error())
	}

	resp := &observation.BlockHeaderResponse{
		Block: &msg,
	}
	return resp, nil
}

func (h *Handler) GetCollectionByID(_ context.Context, req *observation.GetCollectionByIDRequest) (*observation.CollectionResponse, error) {

	id := flow.HashToID(req.Id)

	// retrieve the collection from the collection storage
	cl, err := h.collections.LightByID(id)
	if err != nil {
		err = getError(err, "Collection")
		return nil, err
	}

	transactions := make([]*flow.TransactionBody, len(cl.Transactions))

	// retrieve all transactions from the transaction storage
	for i, txID := range cl.Transactions {
		tx, err := h.transactions.ByID(txID)
		if err != nil {
			err = getError(err, "Collection")
			return nil, err
		}
		transactions[i] = tx
	}

	// create a flow collection object
	collection := &flow.Collection{Transactions: transactions}

	// convert flow collection object to protobuf entity
	ce, err := convert.CollectionToMessage(collection)
	if err != nil {
		err = getError(err, "Collection")
		return nil, err
	}

	// return the collection entity
	resp := &observation.CollectionResponse{
		Collection: ce,
	}
	return resp, nil
}

func (h *Handler) GetLatestBlock(_ context.Context, req *observation.GetLatestBlockRequest) (*observation.BlockResponse, error) {
	var header *flow.Header
	var err error
	if req.IsSealed {
		// get the latest seal header from storage
		header, err = h.getLatestSealedHeader()
	} else {
		// get the finalized header from state
		header, err = h.state.Final().Head()
	}

	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	block, err := h.blocks.ByID(header.ID())
	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	msg, err := convert.BlockToMessage(block)
	if err != nil {
		err = getError(err, "Block")
		return nil, err
	}

	resp := &observation.BlockResponse{
		Block: msg,
	}
	return resp, nil
}

func getError(err error, entity string) error {
	if errors.Is(err, storage.ErrNotFound) {
		return status.Error(codes.NotFound, fmt.Sprintf("%s not found: %v", entity, err))
	}
	return status.Error(codes.Internal, fmt.Sprintf("failed to find %s: %v", entity, err))
}

// deriveTransactionStatus derives the transaction status based on current protocol state
func (h *Handler) deriveTransactionStatus(tx *flow.TransactionBody) (entities.TransactionStatus, error) {

	collection, err := h.collections.LightByTransactionID(tx.ID())

	if errors.Is(err, storage.ErrNotFound) {
		// tx found in transaction storage but not in the collection storage
		return entities.TransactionStatus_STATUS_UNKNOWN, nil
	}
	if err != nil {
		return entities.TransactionStatus_STATUS_UNKNOWN, err
	}

	block, err := h.blocks.ByCollectionID(collection.ID())

	if errors.Is(err, storage.ErrNotFound) {
		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return entities.TransactionStatus_STATUS_PENDING, nil
	}
	if err != nil {
		return entities.TransactionStatus_STATUS_UNKNOWN, err
	}

	// get the latest sealed block from the state
	latestSealedBlock, err := h.getLatestSealedHeader()
	if err != nil {
		return entities.TransactionStatus_STATUS_UNKNOWN, err
	}

	// if the finalized block precedes the latest sealed block, then it can be safely assumed that it would have been
	// sealed as well and the transaction can be considered sealed
	if block.Height <= latestSealedBlock.Height {
		return entities.TransactionStatus_STATUS_SEALED, nil
	}
	// otherwise, the finalized block of the transaction has not yet been sealed
	// hence the transaction is finalized but not sealed
	return entities.TransactionStatus_STATUS_FINALIZED, nil
}
