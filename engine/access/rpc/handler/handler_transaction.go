package handler

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// SendTransaction forwards the transaction to the collection node
func (h *Handler) SendTransaction(ctx context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {

	fmt.Println("GOT TRANSACTION", req.Transaction)

	// send the transaction to the collection node
	resp, err := h.collectionRPC.SendTransaction(ctx, req)
	if err != nil {
		return resp, status.Error(codes.Internal, fmt.Sprintf("failed to send transaction to a collection node: %v", err))
	}

	// convert the request message to a transaction
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	fmt.Println("GOT TRANSACTION 2", tx)

	// store the transaction locally
	err = h.transactions.Store(&tx)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
	}

	return resp, nil
}

func (h *Handler) GetTransaction(_ context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {

	id := flow.HashToID(req.Id)
	// look up transaction from storage
	tx, err := h.transactions.ByID(id)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// convert flow transaction to a protobuf message
	transaction := convert.TransactionToMessage(*tx)

	// return result
	resp := &access.TransactionResponse{
		Transaction: transaction,
	}
	return resp, nil
}

func (h *Handler) GetTransactionResult(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {

	id := flow.HashToID(req.GetId())

	// look up transaction from storage
	tx, err := h.transactions.ByID(id)
	if err != nil {
		return nil, convertStorageError(err)
	}

	txID := tx.ID()

	// derive status of the transaction
	status, err := h.deriveTransactionStatus(tx)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// get events for the transaction
	events, statusCode, txError, err := h.lookupTransactionResult(ctx, txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// TODO: Set correct values for StatusCode and ErrorMessage

	// return result
	resp := &access.TransactionResultResponse{
		Status:       status,
		Events:       events,
		StatusCode:   statusCode,
		ErrorMessage: txError,
	}

	fmt.Println("TX STATUS", resp)

	return resp, nil
}

// deriveTransactionStatus derives the transaction status based on current protocol state
func (h *Handler) deriveTransactionStatus(tx *flow.TransactionBody) (entities.TransactionStatus, error) {

	block, err := h.lookupBlock(tx.ID())
	if err != nil {
		return entities.TransactionStatus_UNKNOWN, err
	}

	if block == nil {
		return entities.TransactionStatus_PENDING, nil
	}

	contains, err := h.state.Sealed().Contains(block.ID())
	if err != nil {
		return entities.TransactionStatus_UNKNOWN, err
	}

	if contains {
		return entities.TransactionStatus_SEALED, nil
	}

	return entities.TransactionStatus_FINALIZED, nil
}

func (h *Handler) lookupBlock(txID flow.Identifier) (*flow.Block, error) {
	collectionIDs, err := h.collections.CollectionIDsByTransactionID(txID)
	if err != nil {
		return nil, err
	}

	for _, collectionID := range collectionIDs {
		blockID, err := h.collections.IsFinalizedByID(collectionID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				continue
			}

			return nil, err
		}

		fmt.Println("TX IS FINALIZED", txID)

		block, err := h.blocks.ByID(blockID)
		if err != nil {
			return nil, err
		}

		return block, nil
	}

	fmt.Println("TX IS NOT FINALIZED", txID)

	return nil, nil
}

func (h *Handler) lookupTransactionResult(ctx context.Context, txID flow.Identifier) ([]*entities.Event, uint32, string, error) {

	// find the block ID for the transaction
	block, err := h.lookupBlock(txID)
	if err != nil {
		return nil, 0, "", convertStorageError(err)
	}

	if block == nil {
		return nil, 0, "", nil
	}

	blockID := block.ID()

	events, txStatus, message, err := h.getTransactionResultFromExecutionNode(ctx, blockID[:], txID[:])
	if err != nil {
		errStatus, _ := status.FromError(err)
		if errStatus.Code() == codes.NotFound {
			return nil, 0, "", nil
		}
	}

	return events, txStatus, message, nil
}
