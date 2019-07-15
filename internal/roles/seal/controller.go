package seal

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sealSvc "github.com/dapperlabs/bamboo-node/grpc/services/seal"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Ping(context.Context, *sealSvc.PingRequest) (*sealSvc.PingResponse, error) {
	return &sealSvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

func (c *Controller) SubmitResultApproval(context.Context, *sealSvc.SubmitResultApprovalRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
