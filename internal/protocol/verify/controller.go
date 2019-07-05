package verify

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	verifySvc "github.com/dapperlabs/bamboo-node/grpc/services/verify"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SubmitExecutionReceipt(context.Context, *verifySvc.SubmitExecutionReceiptRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
