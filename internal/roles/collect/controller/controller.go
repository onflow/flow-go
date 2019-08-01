package controller

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/data"
)

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

func (c *Controller) SubmitCollection(context.Context, *svc.SubmitCollectionRequest) (*svc.SubmitCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *svc.GetTransactionRequest) (*svc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetCollection(context.Context, *svc.GetCollectionRequest) (*svc.GetCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
