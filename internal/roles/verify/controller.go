package verify

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	verifySvc "github.com/dapperlabs/bamboo-node/grpc/services/verify"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/processor"
	// "github.com/dapperlabs/bamboo-node/internal/utils"
)

type Controller struct {
	dal *DAL
	rp  *processor.ReceiptProcessor
}

func NewController(rp *processor.ReceiptProcessor) *Controller {
	return &Controller{
		rp: rp,
	}
}

func (c *Controller) Ping(context.Context, *verifySvc.PingRequest) (*verifySvc.PingResponse, error) {
	return &verifySvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

func (c *Controller) SubmitExecutionReceipt(ctx context.Context, req *verifySvc.SubmitExecutionReceiptRequest) (*empty.Empty, error) {
	// TODO: utils package currently doesn't build, uncomment the lines below when it does. See https://github.com/dapperlabs/bamboo-node/issues/241
	// er := utils.MessageToExecutionReceipt(req.ExecutionReceipt)
	// c.rp.Submit(er, nil)
	return &empty.Empty{}, nil
}
