package observer

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

type Router struct {
	PingHandler *Handler[*access.PingRequest, *access.PingResponse]
}

func NewRouter(observer, upstream access.AccessAPIServer) *Router {
	return &Router{
		PingHandler: &Handler[*accessproto.PingRequest, *accessproto.PingResponse]{
			Name:        "Ping",
			ForwardOnly: true,
			Upstream:    upstream.Ping,
			Observer:    observer.Ping,
		},
	}
}

func (router *Router) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return router.PingHandler.Call(context, req)
}
