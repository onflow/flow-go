package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type handlerTransactions struct {
	collectionRPC      access.AccessAPIClient
	executionRPC       execution.ExecutionAPIClient
	transactions       storage.Transactions
	collections        storage.Collections
	blocks             storage.Blocks
	state              protocol.State
	chainID            flow.ChainID
	transactionMetrics module.TransactionMetrics
}

// SendTransaction forwards the transaction to the collection node
func (h *handlerTransactions) SendTransaction(ctx context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	now := time.Now().UTC()

	// convert the request message to a transaction (has the side effect of validating the address in the tx as well)
	tx, err := convert.MessageToTransaction(req.Transaction, h.chainID.Chain())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	// send the transaction to the collection node if valid
	resp, err := h.collectionRPC.SendTransaction(ctx, req)
	if err != nil {
		return resp, status.Error(codes.Internal, fmt.Sprintf("failed to send transaction to a collection node: %v", err))
	}

	h.transactionMetrics.TransactionReceived(tx.ID(), now)

	// store the transaction locally
	err = h.transactions.Store(&tx)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
	}

	return resp, nil
}

func (h *handlerTransactions) GetTransaction(_ context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {

	reqTxID := req.GetId()
	id, err := convert.TransactionID(reqTxID)
	if err != nil {
		return nil, err
	}

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

func (h *handlerTransactions) GetTransactionResult(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {

	reqTxID := req.GetId()
	id, err := convert.TransactionID(reqTxID)
	if err != nil {
		return nil, err
	}

	// look up transaction from storage
	tx, err := h.transactions.ByID(id)
	if err != nil {
		return nil, convertStorageError(err)
	}

	txID := tx.ID()

	// get events for the transaction
	executed, events, statusCode, txError, err := h.lookupTransactionResult(ctx, txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// derive status of the transaction
	status, err := h.deriveTransactionStatus(tx, executed)
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

	return resp, nil
}

// deriveTransactionStatus derives the transaction status based on current protocol state
func (h *handlerTransactions) deriveTransactionStatus(tx *flow.TransactionBody, executed bool) (entities.TransactionStatus, error) {

	block, err := h.lookupBlock(tx.ID())
	if errors.Is(err, storage.ErrNotFound) {
		// Not in a block, let's see if it's expired
		referenceBlock, err := h.state.AtBlockID(tx.ReferenceBlockID).Head()
		if err != nil {
			return entities.TransactionStatus_UNKNOWN, err
		}
		// get the latest finalized block from the state
		finalized, err := h.state.Final().Head()
		if err != nil {
			return entities.TransactionStatus_UNKNOWN, err
		}

		// Have to check if finalized height is greater than reference block height rather than rely on the subtraction, since
		// heights are unsigned ints
		if finalized.Height > referenceBlock.Height && finalized.Height-referenceBlock.Height > flow.DefaultTransactionExpiry {
			return entities.TransactionStatus_EXPIRED, err
		}

		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return entities.TransactionStatus_PENDING, nil
	}
	if err != nil {
		return entities.TransactionStatus_UNKNOWN, err
	}

	// get the latest sealed block from the state
	sealed, err := h.state.Sealed().Head()
	if err != nil {
		return entities.TransactionStatus_UNKNOWN, err
	}

	// if the finalized block precedes the latest sealed block, then it can be safely assumed that it would have been
	// sealed as well and the transaction can be considered sealed
	if block.Header.Height <= sealed.Height {
		return entities.TransactionStatus_SEALED, nil
	}

	// We've explicitly seen some form of execution result, therefore this Tx must have been executed
	if executed {
		return entities.TransactionStatus_EXECUTED, nil
	}

	// otherwise, the finalized block of the transaction has not yet been sealed
	// hence the transaction is finalized but not sealed
	return entities.TransactionStatus_FINALIZED, nil
}

func (h *handlerTransactions) lookupBlock(txID flow.Identifier) (*flow.Block, error) {

	collection, err := h.collections.LightByTransactionID(txID)
	if err != nil {
		return nil, err
	}

	block, err := h.blocks.ByCollectionID(collection.ID())
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (h *handlerTransactions) lookupTransactionResult(ctx context.Context, txID flow.Identifier) (bool, []*entities.Event, uint32, string, error) {

	// find the block ID for the transaction
	block, err := h.lookupBlock(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// access node may not have the block if it hasn't yet been finalized
			return false, nil, 0, "", nil
		}
		return false, nil, 0, "", convertStorageError(err)
	}

	blockID := block.ID()

	events, txStatus, message, err := h.getTransactionResultFromExecutionNode(ctx, blockID[:], txID[:])
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// No result yet, indicate that it has not been executed
			return false, nil, 0, "", nil
		}
	}
	// considered executed as long as some result is returned, even if it's an error
	return true, events, txStatus, message, nil
}

func (h *handlerTransactions) getTransactionResultFromExecutionNode(ctx context.Context,
	blockID []byte,
	transactionID []byte) ([]*entities.Event, uint32, string, error) {

	// create an execution API request for events at blockID and transactionID
	req := execution.GetTransactionResultRequest{
		BlockId:       blockID,
		TransactionId: transactionID,
	}

	// call the execution node gRPC
	resp, err := h.executionRPC.GetTransactionResult(ctx, &req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, 0, "", err
		}

		return nil, 0, "", status.Errorf(codes.Internal, "failed to retrieve result from execution node: %v", err)
	}

	exeResults := resp.GetEvents()

	return exeResults, resp.GetStatusCode(), resp.GetErrorMessage(), nil
}
