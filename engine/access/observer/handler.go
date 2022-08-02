package observer

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2"
)

type RPCHandlerMetrics interface {
	Record(handler, rpc string, err, fallback bool)
}

type RPCHandler[Request any, Response any] func(ctx context.Context, r Request) (Response, error)

type Handler[Request any, Response any] struct {
	RPCHandlerMetrics

	Name         string // rpc name
	Logger       zerolog.Logger
	ForwardOnly  bool                          // call upstream only
	ForwardRetry bool                          // call upstream after observer fails
	Upstream     RPCHandler[Request, Response] // upstream access handler
	Observer     RPCHandler[Request, Response] // local observer handler
}

func (h *Handler[Request, Response]) Call(ctx context.Context, r Request) (Response, error) {
	if h.ForwardOnly {
		return h.callUpstream(ctx, r, false)
	}

	res, err := h.callObserver(ctx, r)

	if err != nil && h.ForwardRetry {
		return h.callUpstream(ctx, r, false)
	}

	return res, err
}

func (h *Handler[Request, Response]) callUpstream(ctx context.Context, r Request, fallback bool) (Response, error) {
	res, err := h.Upstream(ctx, r)
	h.log("upstream", h.Name, err, fallback)
	h.Record("upstream", h.Name, err != nil, fallback)
	return res, err
}

func (h *Handler[Request, Response]) callObserver(ctx context.Context, r Request) (Response, error) {
	res, err := h.Observer(ctx, r)
	h.log("observer", h.Name, err, false)
	h.Record("observer", h.Name, err != nil, false)
	return res, err
}

func (h *Handler[Request, Response]) log(handler, rpc string, err error, fallback bool) {
	if err != nil {
		h.Logger.Err(err).Str("handler", handler).Str("rpc", rpc).Bool("fallback", fallback)
		return
	}

	h.Logger.Str("handler", handler).Str("rpc", rpc).Bool("fallback", fallback)
}
