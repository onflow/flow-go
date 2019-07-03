package controllers

import (
	"context"

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
