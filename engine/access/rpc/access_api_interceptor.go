package rpc

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type accessAPIInterceptor struct {
	log zerolog.Logger
}

// Authorization unary interceptor function to handle authorize per RPC call
func (interceptor *accessAPIInterceptor) serverInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	start := time.Now()

	// call the handler
	h, err := handler(ctx, req)

	interceptor.log.Debug().
		Str("method", info.FullMethod).
		Interface("request", req).
		Interface("response", resp).
		Err(err).
		Dur("request_duration", time.Since(start))

	return h, err
}
