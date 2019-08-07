package observe

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	observeSvc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Ping(context.Context, *observeSvc.PingRequest) (*observeSvc.PingResponse, error) {
	return &observeSvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

func (c *Controller) SendTransaction(context.Context, *observeSvc.SendTransactionRequest) (*observeSvc.SendTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByHash(context.Context, *observeSvc.GetBlockByHashRequest) (*observeSvc.GetBlockByHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByNumber(context.Context, *observeSvc.GetBlockByNumberRequest) (*observeSvc.GetBlockByNumberResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetLatestBlock(context.Context, *observeSvc.GetLatestBlockRequest) (*observeSvc.GetLatestBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *observeSvc.GetTransactionRequest) (*observeSvc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetAccount(context.Context, *observeSvc.GetAccountRequest) (*observeSvc.GetAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) Call(context.Context, *observeSvc.CallRequest) (*observeSvc.CallResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
