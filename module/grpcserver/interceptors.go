package grpcserver

import (
	"context"

	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// irrecoverableCtxInjector injects the irrecoverable signaler context into requests so that the API
// handler and backend can use the irrecoverable system to handle exceptions.
//
// It injects the signaler context into the original gRPC context via a context value using the key
// irrecoverable.SignalerContextKey. If signalerCtx is not set yet (because the grpc server has not
// finished initializing), the original context is passed unchanged.
func irrecoverableCtxInjector(signalerCtx *atomic.Pointer[irrecoverable.SignalerContext]) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if sigCtx := signalerCtx.Load(); sigCtx != nil {
			resp, err = handler(irrecoverable.WithSignalerContext(ctx, *sigCtx), req)
		} else {
			resp, err = handler(ctx, req)
		}
		return
	}
}
