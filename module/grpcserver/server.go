package grpcserver

import (
	"net"
	"sync"

	"github.com/rs/zerolog"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// GrpcServer defines a grpc server that starts once and uses in different Engines.
// It makes it easy to configure the node to use the same port for both APIs.
type GrpcServer struct {
	component.Component
	log    zerolog.Logger
	Server *grpc.Server

	grpcListenAddr string // the GRPC server address as ip:port

	addrLock    sync.RWMutex
	grpcAddress net.Addr
}

// NewGrpcServer returns a new grpc server.
func NewGrpcServer(log zerolog.Logger,
	grpcListenAddr string,
	grpcServer *grpc.Server,
) *GrpcServer {
	log.Info().Msg("================> NewGrpcServer")
	server := &GrpcServer{
		log:            log,
		Server:         grpcServer,
		grpcListenAddr: grpcListenAddr,
	}
	cm := component.ComponentManagerBuilderImpl{Log: log}
	server.Component = cm.
		AddWorker(server.serveGRPCWorker).
		AddWorker(server.shutdownWorker).
		Build()
	log.Info().Msg("================> End NewGrpcServer")
	return server
}

// serveGRPCWorker is a worker routine which starts the gRPC server.
// The ready callback is called after the server address is bound and set.
func (g *GrpcServer) serveGRPCWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	g.log = g.log.With().Str("grpc_address", g.grpcListenAddr).Logger()
	g.log.Info().Msg("================> serveGRPCWorker")
	g.log.Info().Msg("starting grpc server on address")

	l, err := net.Listen("tcp", g.grpcListenAddr)
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
	g.log.Debug().Msg("listening on port")
	ready()

	err = g.Server.Serve(l) // blocking call
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
	g.Server.GracefulStop()
}
