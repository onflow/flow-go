package grpcserver

import (
	"context"

	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// IrrecoverableCtxInjector injects the irrecoverable signaler context into requests so that the API
// handler and backend can use the irrecoverable system to handle exceptions.
//
// It injects the signaler context into the original gRPC context via a context value using the key
// irrecoverable.SignalerContextKey. If signalerCtx is not set yet (because the grpc server has not
// finished initializing), the original context is passed unchanged.
func IrrecoverableCtxInjector(signalerCtx *atomic.Pointer[irrecoverable.SignalerContext]) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// signalerCtx is set by the server initialization logic. in practice, the context should
		// always be set by the time the first request is received since it is added before starting
		// the server.
		if sigCtx := signalerCtx.Load(); sigCtx != nil {
			return handler(irrecoverable.WithSignalerContext(ctx, *sigCtx), req)
		}

		// If signalerCtx is not available yet, just pass through the original context.
		// This is OK since the `irrecoverable.Throw` function will still cause the node to crash
		// even if it is passed a regular context.
		return handler(ctx, req)
	}
}
