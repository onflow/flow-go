package observe

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	observeSvc "github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
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

func (c *Controller) GetLatestBlock(context.Context, *observeSvc.GetLatestBlockRequest) (*observeSvc.GetLatestBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetTransaction(context.Context, *observeSvc.GetTransactionRequest) (*observeSvc.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetAccount(context.Context, *observeSvc.GetAccountRequest) (*observeSvc.GetAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) CallScript(context.Context, *observeSvc.CallScriptRequest) (*observeSvc.CallScriptResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
