package collect

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	collectSvc "github.com/dapperlabs/bamboo-node/grpc/services/collect"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SubmitTransaction(context.Context, *collectSvc.SubmitTransactionRequest) (*collectSvc.SubmitTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) SubmitCollection(context.Context, *collectSvc.SubmitCollectionRequest) (*collectSvc.SubmitCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *collectSvc.GetTransactionRequest) (*collectSvc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetCollection(context.Context, *collectSvc.GetCollectionRequest) (*collectSvc.GetCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
