package controllers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/access/data"
)

// Controller implements AccessNodeServer interface
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

// UserSubmitTransaction .
func (c *Controller) UserSubmitTransaction(context.Context, *bambooProto.UserTransactionRequest) (*bambooProto.UserTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ClusterSubmitTransaction .
func (c *Controller) ClusterSubmitTransaction(context.Context, *bambooProto.ClusterTransactionRequest) (*bambooProto.ClusterTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetTransaction .
func (c *Controller) GetTransaction(context.Context, *bambooProto.TransactionRequest) (*bambooProto.TransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetCollection .
func (c *Controller) GetCollection(context.Context, *bambooProto.CollectionRequest) (*bambooProto.CollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// UpdateTransaction .
func (c *Controller) UpdateTransaction(context.Context, *bambooProto.TransactionUpdateRequest) (*bambooProto.TransactionUpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// UpdateCollection .
func (c *Controller) UpdateCollection(context.Context, *bambooProto.CollectionUpdateRequest) (*bambooProto.CollectionUpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetRegisters .
func (c *Controller) GetRegisters(context.Context, *bambooProto.RegistersRequest) (*bambooProto.RegistersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetBalance .
func (c *Controller) GetBalance(context.Context, *bambooProto.BalanceRequest) (*bambooProto.BalanceResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ProcessExecutionReceipt .
func (c *Controller) ProcessExecutionReceipt(context.Context, *bambooProto.ProcessExecutionReceiptRequest) (*bambooProto.ProcessExecutionReceiptResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
