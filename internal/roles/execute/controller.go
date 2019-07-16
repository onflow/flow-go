package execute

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	executeSvc "github.com/dapperlabs/bamboo-node/grpc/services/execute"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Ping(context.Context, *executeSvc.PingRequest) (*executeSvc.PingResponse, error) {
	return &executeSvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

func (c *Controller) ExecuteBlock(context.Context, *executeSvc.ExecuteBlockRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) NotifyBlockExecuted(context.Context, *executeSvc.NotifyBlockExecutedRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetRegisters(context.Context, *executeSvc.GetRegistersRequest) (*executeSvc.GetRegistersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetRegistersAtBlockHeight(context.Context, *executeSvc.GetRegistersAtBlockHeightRequest) (*executeSvc.GetRegistersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
