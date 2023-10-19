package grpcserver

import (
	"net"
	"sync"

	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" //required for gRPC compression

	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/deflate" // required for gRPC compression
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"  // required for gRPC compression

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// GrpcServer wraps `grpc.Server` and allows to manage it using `component.Component` interface. It can be injected
// into different engines making it possible to use single grpc server for multiple services which live in different modules.
type GrpcServer struct {
	component.Component
	log    zerolog.Logger
	Server *grpc.Server

	grpcListenAddr string // the GRPC server address as ip:port

	addrLock    sync.RWMutex
	grpcAddress net.Addr
}

var _ component.Component = (*GrpcServer)(nil)

// NewGrpcServer returns a new grpc server.
func NewGrpcServer(log zerolog.Logger,
	grpcListenAddr string,
	grpcServer *grpc.Server,
) *GrpcServer {
	server := &GrpcServer{
		log:            log,
		Server:         grpcServer,
		grpcListenAddr: grpcListenAddr,
	}
	server.Component = component.NewComponentManagerBuilder().
		AddWorker(server.serveGRPCWorker).
		AddWorker(server.shutdownWorker).
		Build()
	return server
}

// serveGRPCWorker is a worker routine which starts the gRPC server.
// The ready callback is called after the server address is bound and set.
func (g *GrpcServer) serveGRPCWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	g.log = g.log.With().Str("grpc_address", g.grpcListenAddr).Logger()
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
