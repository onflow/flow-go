package access

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessSvc "github.com/dapperlabs/bamboo-node/grpc/services/access"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SendTransaction(context.Context, *accessSvc.SendTransactionRequest) (*accessSvc.SendTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByHash(context.Context, *accessSvc.GetBlockByHashRequest) (*accessSvc.GetBlockByHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByNumber(context.Context, *accessSvc.GetBlockByNumberRequest) (*accessSvc.GetBlockByNumberResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetLatestBlock(context.Context, *accessSvc.GetLatestBlockRequest) (*accessSvc.GetLatestBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *accessSvc.GetTransactionRequest) (*accessSvc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetAccount(context.Context, *accessSvc.GetAccountRequest) (*accessSvc.GetAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) CallContract(context.Context, *accessSvc.CallContractRequest) (*accessSvc.CallContractResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
