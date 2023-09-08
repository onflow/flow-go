package grpcserver

import (
	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"

	"github.com/onflow/flow-go/engine/common/rpc"
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

	transportCredentials         credentials.TransportCredentials // the GRPC credentials
	stateStreamInterceptorEnable bool
}

// NewGrpcServerBuilder helps to build a new grpc server.
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

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(maxMsgSize)),
		grpc.MaxSendMsgSize(int(maxMsgSize)),
	}
	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
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

	return grpcServerBuilder
}

func (b *GrpcServerBuilder) Build() *GrpcServer {
	return NewGrpcServer(b.log, b.gRPCListenAddr, b.server)
}
