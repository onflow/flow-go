package controllers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/execute/data"
)

// Controller implements ExecuteNodeServer interface
type Controller struct {
	dal *data.DAL
}

// NewController ..
func NewController(dal *data.DAL) *Controller {
	return &Controller{dal: dal}
}

// Ping  .
func (c *Controller) Ping(context.Context, *bambooProto.PingRequest) (*bambooProto.PingResponse, error) {
	return &bambooProto.PingResponse{
		Address: []byte("ping pong!"),
	}, nil
}

// ExecuteBlock .
func (c *Controller) ExecuteBlock(context.Context, *bambooProto.ExecuteBlockRequest) (*bambooProto.ExecuteBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NotifyBlockExecuted .
func (c *Controller) NotifyBlockExecuted(context.Context, *bambooProto.NotifyBlockExecutedRequest) (*bambooProto.NotifyBlockExecutedResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetRegisters provides register values and metadata.
func (c *Controller) GetRegisters(context.Context, *bambooProto.RegistersRequest) (*bambooProto.RegistersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetRegistersAtBlockHeight .
func (c *Controller) GetRegistersAtBlockHeight(context.Context, *bambooProto.RegistersAtBlockHeightRequest) (*bambooProto.RegistersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
