package ping

import (
	"context"

	pingSvc "github.com/dapperlabs/bamboo-node/grpc/services/ping"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Ping(context.Context, *pingSvc.PingRequest) (*pingSvc.PingResponse, error) {
	return &pingSvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}
