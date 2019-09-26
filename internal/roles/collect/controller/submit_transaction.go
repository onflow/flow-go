package controller

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svc "github.com/dapperlabs/flow-go/pkg/grpc/services/collect"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/pkg/types/proto"
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
	tx, err := proto.MessageToTransaction(req.GetTransaction())
	if err != nil {
		if err == proto.ErrEmptyMessage {
			return nil, status.Error(codes.InvalidArgument, msgEmptyTransaction)
		}

		return nil, status.Error(codes.InvalidArgument, msgFailedTransactionDecode)
	}

	if c.storage.ContainsTransaction(tx.Hash()) {
		return &svc.SubmitTransactionResponse{}, nil
	}

	if err := validateTransaction(tx); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// TODO: validate transaction signature
	// https://github.com/dapperlabs/flow-go/issues/171

	// TODO: route transaction to cluster if required and return

	c.txPool.Insert(tx)

	err = c.storage.InsertTransaction(tx)
	if err != nil {
		return nil, status.Error(codes.Internal, msgInternalError)
	}

	return &svc.SubmitTransactionResponse{}, nil
}

// InvalidTransactionError indicates that a transaction does not contain all
// required information.
type InvalidTransactionError struct {
	missingFields []string
}

func (e InvalidTransactionError) Error() string {
	return fmt.Sprintf(
		"required fields are not set: %s",
		strings.Join(e.missingFields[:], ", "),
	)
}

// validateTransaction returns an error if the transaction does not contain all required fields.
func validateTransaction(tx types.Transaction) error {
	missingFields := make([]string, 0)

	if len(tx.Script) == 0 {
		missingFields = append(missingFields, "script")
	}

	if tx.ComputeLimit == 0 {
		missingFields = append(missingFields, "compute_limit")
	}

	if len(missingFields) > 0 {
		return InvalidTransactionError{missingFields}
	}

	return nil
}
