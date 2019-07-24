package verify

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	verifySvc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/verify"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Ping(context.Context, *verifySvc.PingRequest) (*verifySvc.PingResponse, error) {
	return &verifySvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

func (c *Controller) SubmitExecutionReceipt(context.Context, *verifySvc.SubmitExecutionReceiptRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
