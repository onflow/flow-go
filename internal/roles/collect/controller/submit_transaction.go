package controller

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"

	"github.com/dapperlabs/bamboo-node/pkg/types/proto"
)

const (
	msgEmptyTransaction        = "transaction field is empty"
	msgFailedTransactionDecode = "failed to decode transaction"
	msgDuplicateTransaction    = "transaction has already been submitted"
)

// SubmitTransaction accepts an incoming transaction from a user agent or peer node.
//
// This function will return an error in the follow cases:
//
// The request is malformed or incomplete.
// The transaction has an invalid or missing signature.
//
// The submitted transaction will be stored for future inclusion in a collection
// if it belongs to this node's cluster, but otherwise it will be forwarded to the
// correct cluster.
func (c *Controller) SubmitTransaction(
	ctx context.Context, req *svc.SubmitTransactionRequest,
) (*svc.SubmitTransactionResponse, error) {
	tx, err := proto.MessageToSignedTransaction(req.GetTransaction())
	if err != nil {
		if err == proto.ErrEmptyMessage {
			return nil, status.Error(codes.InvalidArgument, msgEmptyTransaction)
		}

		return nil, status.Error(codes.InvalidArgument, msgFailedTransactionDecode)
	}

	if c.storage.ContainsTransaction(tx.Hash()) {
		return &svc.SubmitTransactionResponse{}, nil
	}

	if err := tx.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// TODO: validate transaction signature
	// https://github.com/dapperlabs/bamboo-node/issues/171

	c.txPool.Insert(tx)

	err = c.storage.InsertTransaction(tx)
	if err != nil {
		return nil, status.Error(codes.Internal, msgInternalError)
	}

	// TODO: route transaction to cluster

	return &svc.SubmitTransactionResponse{}, nil
}
