package grpcserver

import (
	"context"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog"

	"go.uber.org/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Option func(*GrpcServerBuilder)

// WithTransportCredentials sets the transport credentials parameters for a grpc server builder.
func WithTransportCredentials(transportCredentials credentials.TransportCredentials) Option {
	return func(c *GrpcServerBuilder) {
		c.transportCredentials = transportCredentials
	}
}

// WithStreamInterceptor sets the StreamInterceptor option to grpc server.
func WithStreamInterceptor() Option {
	return func(c *GrpcServerBuilder) {
		c.stateStreamInterceptorEnable = true
	}
}

// GrpcServerBuilder created for separating the creation and starting GrpcServer,
// cause services need to be registered before the server starts.
type GrpcServerBuilder struct {
	log            zerolog.Logger
	gRPCListenAddr string
	server         *grpc.Server
	signalerCtx    *atomic.Pointer[irrecoverable.SignalerContext]

	transportCredentials         credentials.TransportCredentials // the GRPC credentials
	stateStreamInterceptorEnable bool
}

// NewGrpcServerBuilder creates a new builder for configuring and initializing a gRPC server.
//
// The builder is configured with the provided parameters such as logger, gRPC server address, maximum message size,
// API rate limits, and additional options. The builder also sets up the necessary interceptors, including handling
// irrecoverable errors using the irrecoverable.SignalerContext. The gRPC server can be configured with options such
// as maximum message sizes and interceptors for handling RPC calls.
//
// If RPC metrics are enabled, the builder adds the gRPC Prometheus interceptor for collecting metrics. Additionally,
// it can enable a state stream interceptor based on the configuration. Rate limiting interceptors can be added based
// on specified API rate limits. Logging and custom interceptors are applied, and the final gRPC server is returned.
//
// If transport credentials are provided, a secure gRPC server is created; otherwise, an unsecured server is initialized.
//
// Note: The gRPC server is created with the specified options and is ready for further configuration or starting.
func NewGrpcServerBuilder(log zerolog.Logger,
	gRPCListenAddr string,
	maxMsgSize uint,
	rpcMetricsEnabled bool,
	apiRateLimits map[string]int, // the api rate limit (max calls per second) for each of the Access API e.g. Ping->100, GetTransaction->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the Access API e.g. Ping->50, GetTransaction->10
	opts ...Option,
) *GrpcServerBuilder {
	log = log.With().Str("component", "grpc_server").Logger()

	grpcServerBuilder := &GrpcServerBuilder{
		gRPCListenAddr: gRPCListenAddr,
	}

	for _, applyOption := range opts {
		applyOption(grpcServerBuilder)
	}

	signalerCtx := atomic.NewPointer[irrecoverable.SignalerContext](nil)

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(maxMsgSize)),
		grpc.MaxSendMsgSize(int(maxMsgSize)),
	}
	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
	// This interceptor is responsible for ensuring that irrecoverable errors are properly propagated using
	// the irrecoverable.SignalerContext. It replaces the original gRPC context with a new one that includes
	// the irrecoverable.SignalerContextKey if available, allowing the server to handle error conditions indicating
	// an inconsistent or corrupted node state. If no irrecoverable.SignalerContext is present, the original context
	// is used to process the gRPC request.
	//
	// The interceptor follows the grpc.UnaryServerInterceptor signature, where it takes the incoming gRPC context,
	// request, unary server information, and handler function. It returns the response and error after handling
	// the request. This mechanism ensures consistent error handling for gRPC requests across the server.
	interceptors = append(interceptors, func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if signalerCtx := signalerCtx.Load(); signalerCtx != nil {
			valueCtx := context.WithValue(ctx, irrecoverable.SignalerContextKey{}, *signalerCtx)
			resp, err = handler(valueCtx, req)
		} else {
			resp, err = handler(ctx, req)
		}
		return
	})

	// if rpc metrics is enabled, first create the grpc metrics interceptor
	if rpcMetricsEnabled {
		interceptors = append(interceptors, grpc_prometheus.UnaryServerInterceptor)

		if grpcServerBuilder.stateStreamInterceptorEnable {
			// note: intentionally not adding logging or rate limit interceptors for streams.
			// rate limiting is done in the handler, and we don't need log events for every message as
			// that would be too noisy.
			log.Info().Msg("stateStreamInterceptorEnable true")
			grpcOpts = append(grpcOpts, grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor))
		} else {
			log.Info().Msg("stateStreamInterceptorEnable false")
		}
	}
	if len(apiRateLimits) > 0 {
		// create a rate limit interceptor
		rateLimitInterceptor := rpc.NewRateLimiterInterceptor(log, apiRateLimits, apiBurstLimits).UnaryServerInterceptor
		// append the rate limit interceptor to the list of interceptors
		interceptors = append(interceptors, rateLimitInterceptor)
	}
	// add the logging interceptor, ensure it is innermost wrapper
	interceptors = append(interceptors, rpc.LoggingInterceptor(log)...)
	// create a chained unary interceptor
	// create an unsecured grpc server
	grpcOpts = append(grpcOpts, grpc.ChainUnaryInterceptor(interceptors...))

	if grpcServerBuilder.transportCredentials != nil {
		log = log.With().Str("endpoint", "secure").Logger()
		// create a secure server by using the secure grpc credentials that are passed in as part of config
		grpcOpts = append(grpcOpts, grpc.Creds(grpcServerBuilder.transportCredentials))
	} else {
		log = log.With().Str("endpoint", "unsecure").Logger()
	}
	grpcServerBuilder.log = log
	grpcServerBuilder.server = grpc.NewServer(grpcOpts...)
	grpcServerBuilder.signalerCtx = signalerCtx

	return grpcServerBuilder
}

func (b *GrpcServerBuilder) Build() *GrpcServer {
	return NewGrpcServer(b.log, b.gRPCListenAddr, b.server, b.signalerCtx)
}
