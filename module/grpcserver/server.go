package grpcserver

import (
	"net"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// GrpcServerConfig defines the configurable options for the access node server
// GRPC server here implies a server that presents a self-signed TLS certificate and a client that authenticates
// the server via a pre-shared public key
type GrpcServerConfig struct {
	GRPCListenAddr       string                           // the GRPC server address as ip:port
	TransportCredentials credentials.TransportCredentials // the GRPC credentials
	MaxMsgSize           uint                             // GRPC max message size
}

// NewGrpcServerConfig initializes a new grpc server config.
func NewGrpcServerConfig(grpcListenAddr string, maxMsgSize uint, opts ...Option) GrpcServerConfig {
	server := GrpcServerConfig{
		GRPCListenAddr: grpcListenAddr,
		MaxMsgSize:     maxMsgSize,
	}
	for _, applyOption := range opts {
		applyOption(&server)
	}

	return server
}

type Option func(*GrpcServerConfig)

// WithTransportCredentials sets the transport credentials parameters for a grpc server config.
func WithTransportCredentials(transportCredentials credentials.TransportCredentials) Option {
	return func(c *GrpcServerConfig) {
		c.TransportCredentials = transportCredentials
	}
}

// GrpcServer defines a grpc server that starts once and uses in different Engines.
// It makes it easy to configure the node to use the same port for both APIs.
type GrpcServer struct {
	component.Component
	log        zerolog.Logger
	cm         *component.ComponentManager
	grpcServer *grpc.Server

	config GrpcServerConfig

	addrLock    sync.RWMutex
	grpcAddress net.Addr
}

// NewGrpcServer returns a new grpc server.
func NewGrpcServer(log zerolog.Logger,
	config GrpcServerConfig,
	grpcServer *grpc.Server,
) (*GrpcServer, error) {
	server := &GrpcServer{
		log:        log,
		grpcServer: grpcServer,
		config:     config,
	}
	server.cm = component.NewComponentManagerBuilder().
		AddWorker(server.serveGRPCWorker).
		AddWorker(server.shutdownWorker).
		Build()
	server.Component = server.cm
	return server, nil
}

// serveGRPCWorker is a worker routine which starts the gRPC server.
// The ready callback is called after the server address is bound and set.
func (g *GrpcServer) serveGRPCWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	g.log.Info().Str("grpc_address", g.config.GRPCListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", g.config.GRPCListenAddr)
	if err != nil {
		g.log.Err(err).Msg("failed to start the grpc server")
		ctx.Throw(err)
		return
	}

	// save the actual address on which we are listening (may be different from g.config.GRPCListenAddr if not port
	// was specified)
	g.addrLock.Lock()
	g.grpcAddress = l.Addr()
	g.addrLock.Unlock()
	g.log.Debug().Str("grpc_address", g.grpcAddress.String()).Msg("listening on port")
	ready()

	err = g.grpcServer.Serve(l) // blocking call
	if err != nil {
		g.log.Err(err).Msg("fatal error in grpc server")
		ctx.Throw(err)
	}
}

// GRPCAddress returns the listen address of the GRPC server.
// Guaranteed to be non-nil after Engine.Ready is closed.
func (g *GrpcServer) GRPCAddress() net.Addr {
	g.addrLock.RLock()
	defer g.addrLock.RUnlock()
	return g.grpcAddress
}

// shutdownWorker is a worker routine which shuts down server when the context is cancelled.
func (g *GrpcServer) shutdownWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	<-ctx.Done()
	g.grpcServer.GracefulStop()
}

type GrpcServerBuilder struct {
	log    zerolog.Logger
	config GrpcServerConfig
	server *grpc.Server
}

// NewGrpcServerBuilder helps to build a new grpc server.
func NewGrpcServerBuilder(log zerolog.Logger,
	config GrpcServerConfig,
	rpcMetricsEnabled bool,
	apiRateLimits map[string]int, // the api rate limit (max calls per second) for each of the Access API e.g. Ping->100, GetTransaction->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the Access API e.g. Ping->50, GetTransaction->10
) *GrpcServerBuilder {
	log = log.With().Str("component", "grpc_server").Logger()
	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(config.MaxMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxMsgSize)),
	}
	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
	// if rpc metrics is enabled, first create the grpc metrics interceptor
	if rpcMetricsEnabled {
		interceptors = append(interceptors, grpc_prometheus.UnaryServerInterceptor)
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
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	// create an unsecured grpc server
	grpcOpts = append(grpcOpts, chainedInterceptors)
	if config.TransportCredentials != nil {
		// create a secure server by using the secure grpc credentials that are passed in as part of config
		grpcOpts = append(grpcOpts, grpc.Creds(config.TransportCredentials))
	}

	return &GrpcServerBuilder{
		log:    log,
		config: config,
		server: grpc.NewServer(grpcOpts...),
	}
}

func (b *GrpcServerBuilder) Server() *grpc.Server {
	return b.server
}

func (b *GrpcServerBuilder) Build() (*GrpcServer, error) {
	return NewGrpcServer(b.log, b.config, b.server)
}
