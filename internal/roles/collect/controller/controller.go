package controller

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/data"
)

const errNoTransaction = "request must include a transaction"

type Controller struct {
	dal *data.DAL
	log *logrus.Entry
}

func New(log *logrus.Logger) *Controller {
	return &Controller{
		log: logrus.NewEntry(log),
	}
}

func (c *Controller) Ping(context.Context, *svc.PingRequest) (*svc.PingResponse, error) {
	return &svc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

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
	tx := req.GetTransaction()
	if tx == nil {
		return nil, status.Error(codes.InvalidArgument, errNoTransaction)
	}

	// TODO: validate transaction contents
	// https://github.com/dapperlabs/bamboo-node/issues/170

	// TODO: validate transaction signature
	// https://github.com/dapperlabs/bamboo-node/issues/171

	// TODO: store transaction
	// https://github.com/dapperlabs/bamboo-node/issues/169

	// TODO: route transaction to cluster

	return &svc.SubmitTransactionResponse{}, nil
}

func (c *Controller) SubmitCollection(context.Context, *svc.SubmitCollectionRequest) (*svc.SubmitCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *svc.GetTransactionRequest) (*svc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetCollection(context.Context, *svc.GetCollectionRequest) (*svc.GetCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
