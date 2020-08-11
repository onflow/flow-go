package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/access"
	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type backendTransactions struct {
	collectionRPC      accessproto.AccessAPIClient
	executionRPC       execproto.ExecutionAPIClient
	transactions       storage.Transactions
	collections        storage.Collections
	blocks             storage.Blocks
	state              protocol.State
	chainID            flow.ChainID
	transactionMetrics module.TransactionMetrics
	retry              *Retry
}

// SendTransaction forwards the transaction to the collection node
func (b *backendTransactions) SendTransaction(
	ctx context.Context,
	tx *flow.TransactionBody,
) error {
	now := time.Now().UTC()

	colReq := &accessproto.SendTransactionRequest{
		Transaction: convert.TransactionToMessage(*tx),
	}

	// send the transaction to the collection node if valid
	_, err := b.collectionRPC.SendTransaction(ctx, colReq)
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to send transaction to a collection node: %v", err))
	}

	b.transactionMetrics.TransactionReceived(tx.ID(), now)

	// store the transaction locally
	err = b.transactions.Store(tx)
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
	}

	go b.registerTransactionForRetry(tx)

	return nil
}

// SendRawTransaction sends a raw transaction to the collection node
func (b *backendTransactions) SendRawTransaction(
	ctx context.Context,
	tx *entitiesproto.Transaction,
) (*accessproto.SendTransactionResponse, error) {
	req := &accessproto.SendTransactionRequest{
		Transaction: tx,
	}

	// send the transaction to the collection node
	return b.collectionRPC.SendTransaction(ctx, req)
}

func (b *backendTransactions) GetTransaction(_ context.Context, txID flow.Identifier) (*flow.TransactionBody, error) {
	// look up transaction from storage
	tx, err := b.transactions.ByID(txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	return tx, nil
}

func (b *backendTransactions) GetTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (*access.TransactionResult, error) {
	// look up transaction from storage
	tx, err := b.transactions.ByID(txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// get events for the transaction
	executed, events, statusCode, txError, err := b.lookupTransactionResult(ctx, txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// derive status of the transaction
	status, err := b.DeriveTransactionStatus(tx, executed)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// TODO: Set correct values for StatusCode and ErrorMessage

	return &access.TransactionResult{
		Status:       status,
		StatusCode:   uint(statusCode),
		Events:       events,
		ErrorMessage: txError,
	}, nil
}

// DeriveTransactionStatus derives the transaction status based on current protocol state
func (b *backendTransactions) DeriveTransactionStatus(
	tx *flow.TransactionBody,
	executed bool,
) (flow.TransactionStatus, error) {

	block, err := b.lookupBlock(tx.ID())
	if errors.Is(err, storage.ErrNotFound) {
		// Not in a block, let's see if it's expired
		referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}
		// get the latest finalized block from the state
		finalized, err := b.state.Final().Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}

		// Have to check if finalized height is greater than reference block height rather than rely on the subtraction, since
		// heights are unsigned ints
		if finalized.Height > referenceBlock.Height && finalized.Height-referenceBlock.Height > flow.DefaultTransactionExpiry {
			return flow.TransactionStatusExpired, err
		}

		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return flow.TransactionStatusPending, nil
	}
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}

	if !executed {
		// If we've gotten here, but the block has not yet been executed, report it as only been finalized
		return flow.TransactionStatusFinalized, nil
	}

	// From this point on, we know for sure this transaction has at least been executed

	// get the latest sealed block from the state
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}

	if block.Header.Height > sealed.Height {
		// The block is not yet sealed, so we'll report it as only executed
		return flow.TransactionStatusExecuted, nil
	}

	// otherwise, this block has been executed, and sealed, so report as sealed
	return flow.TransactionStatusSealed, nil
}

func (b *backendTransactions) lookupBlock(txID flow.Identifier) (*flow.Block, error) {

	collection, err := b.collections.LightByTransactionID(txID)
	if err != nil {
		return nil, err
	}

	block, err := b.blocks.ByCollectionID(collection.ID())
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (b *backendTransactions) lookupTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (bool, []flow.Event, uint32, string, error) {

	// find the block ID for the transaction
	block, err := b.lookupBlock(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// access node may not have the block if it hasn't yet been finalized
			return false, nil, 0, "", nil
		}
		return false, nil, 0, "", convertStorageError(err)
	}

	blockID := block.ID()

	events, txStatus, message, err := b.getTransactionResultFromExecutionNode(ctx, blockID[:], txID[:])
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// No result yet, indicate that it has not been executed
			return false, nil, 0, "", nil
		}
	}

	// considered executed as long as some result is returned, even if it's an error
	return true, events, txStatus, message, nil
}

func (b *backendTransactions) registerTransactionForRetry(tx *flow.TransactionBody) {
	referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
	if err != nil {
		return
	}

	b.retry.RegisterTransaction(referenceBlock.Height, tx)
}

func (b *backendTransactions) getTransactionResultFromExecutionNode(
	ctx context.Context,
	blockID []byte,
	transactionID []byte,
) ([]flow.Event, uint32, string, error) {

	// create an execution API request for events at blockID and transactionID
	req := execproto.GetTransactionResultRequest{
		BlockId:       blockID,
		TransactionId: transactionID,
	}

	// call the execution node gRPC
	resp, err := b.executionRPC.GetTransactionResult(ctx, &req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, 0, "", err
		}

		return nil, 0, "", status.Errorf(codes.Internal, "failed to retrieve result from execution node: %v", err)
	}

	events := convert.MessagesToEvents(resp.GetEvents())

	return events, resp.GetStatusCode(), resp.GetErrorMessage(), nil
}

func (b *backendTransactions) NotifyFinalizedBlockHeight(height uint64) {
	b.retry.Retry(height)
}
