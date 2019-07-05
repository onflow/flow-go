package observation

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	observationSvc "github.com/dapperlabs/bamboo-node/grpc/services/observation"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SendTransaction(context.Context, *observationSvc.SendTransactionRequest) (*observationSvc.SendTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByHash(context.Context, *observationSvc.GetBlockByHashRequest) (*observationSvc.GetBlockByHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByNumber(context.Context, *observationSvc.GetBlockByNumberRequest) (*observationSvc.GetBlockByNumberResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetLatestBlock(context.Context, *observationSvc.GetLatestBlockRequest) (*observationSvc.GetLatestBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *observationSvc.GetTransactionRequest) (*observationSvc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetAccount(context.Context, *observationSvc.GetAccountRequest) (*observationSvc.GetAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) CallContract(context.Context, *observationSvc.CallContractRequest) (*observationSvc.CallContractResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
