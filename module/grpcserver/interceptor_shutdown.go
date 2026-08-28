package grpcserver

import (
	"context"

	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// ShutdownStreamInterceptor cancels the per-stream context as soon as the node's
// [irrecoverable.SignalerContext] is cancelled. This lets long-lived streaming RPCs
// (e.g. block subscriptions) observe shutdown and return promptly, which in turn allows
// [grpc.Server.GracefulStop] to finish without waiting for the client to disconnect.
//
// Without this interceptor, `stream.Context()` is only cancelled when the underlying
// transport dies; [grpc.Server.GracefulStop] does not cancel it. That is why active
// subscriptions block shutdown until either the client disconnects or the server is
// force-stopped via [grpc.Server.Stop].
//
// If `signalerCtx` has not been populated yet (the server is still initializing), the
// stream is passed through unchanged.
func ShutdownStreamInterceptor(signalerCtx *atomic.Pointer[irrecoverable.SignalerContext]) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		sigCtx := signalerCtx.Load()
		if sigCtx == nil {
			return handler(srv, ss)
		}

		ctx, cancel := context.WithCancel(ss.Context())
		defer cancel()

		// Fan the SignalerContext's Done into the stream's context. The watcher exits either
		// when shutdown begins (SignalerContext cancelled) or when the handler returns
		// (defer cancel() above), so no goroutine is leaked.
		go func() {
			select {
			case <-(*sigCtx).Done():
				cancel()
			case <-ctx.Done():
			}
		}()

		return handler(srv, &shutdownAwareStream{ServerStream: ss, ctx: ctx})
	}
}

// shutdownAwareStream wraps a [grpc.ServerStream] so that its Context reflects both the
// original transport-level cancellation and node shutdown.
type shutdownAwareStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the shutdown-aware context for the stream.
func (s *shutdownAwareStream) Context() context.Context {
	return s.ctx
}
