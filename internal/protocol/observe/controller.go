package observe

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	observeSvc "github.com/dapperlabs/bamboo-node/grpc/services/observe"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
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

func (c *Controller) CallContract(context.Context, *observeSvc.CallContractRequest) (*observeSvc.CallContractResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
