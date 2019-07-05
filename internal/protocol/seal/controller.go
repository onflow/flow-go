package seal

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sealSvc "github.com/dapperlabs/bamboo-node/grpc/services/seal"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SubmitResultApproval(context.Context, *sealSvc.SubmitResultApprovalRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
