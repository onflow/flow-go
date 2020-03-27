package rpc

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/storage"
)

// SendTransaction forwards the transaction to the collection node
func (h *Handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {

	// convert the request message to a transaction
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	// TODO: add transaction validation here

	// find the right collection node to send the transaction to
	targetCollNode, err := h.findCollectionNode(tx)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to find collection node for the transacton: %v", err))
	}

	// dial the collection node
	collectionConn, err := grpc.Dial(targetCollNode.Address, h.dialOptions...)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to dial the collection node for the transaction: %v", err))
	}

	collectionRPC := observation.NewObserveServiceClient(collectionConn)

	// send the transaction to the collection node
	resp, err := collectionRPC.SendTransaction(ctx, req)
	if err != nil {
		return resp, err
	}

	// store the transaction locally
	err = h.transactions.Store(&tx)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
	}

	return resp, nil
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

// findCollectionNode finds the collection node that the given transaction should be sent to
func (h *Handler) findCollectionNode(tx flow.TransactionBody) (*flow.Identity, error) {
	// cluster the collection nodes into the configured amount of clusters
	clusters, err := h.state.Final().Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	txCluster := clusters.ByTxID(tx.ID())
	targetID := txCluster.Sample(1)[0]
	return targetID, nil
}
