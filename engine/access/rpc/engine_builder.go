package rpc

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"

	"github.com/onflow/flow-go/access"

	legacyaccess "github.com/onflow/flow-go/access/legacy"
)

type RPCEngineBuilder struct {
	*Engine
	// Use the parent interface instead of implementation, so that we can assign it to proxy.
	handler accessproto.AccessAPIServer
}

// NewRPCEngineBuilder helps to build a new RPC engine.
func NewRPCEngineBuilder(engine *Engine) *RPCEngineBuilder {
	return &RPCEngineBuilder{
		Engine: engine,
		// default handler will use the engine.backend implementation
		handler: access.NewHandler(engine.backend, engine.chain),
	}
}

func (builder *RPCEngineBuilder) WithNewHandler(handler accessproto.AccessAPIClient) {
	builder.handler = &Forwarder{UpstreamHandler: handler}
}

func (builder *RPCEngineBuilder) WithLegacy() {
	// Register legacy gRPC handlers for backwards compatibility, to be removed at a later date
	legacyaccessproto.RegisterAccessAPIServer(
		builder.unsecureGrpcServer,
		legacyaccess.NewHandler(builder.backend, builder.chain),
	)
	legacyaccessproto.RegisterAccessAPIServer(
		builder.secureGrpcServer,
		legacyaccess.NewHandler(builder.backend, builder.chain),
	)
}

func (builder *RPCEngineBuilder) WithMetrics() {
	// Not interested in legacy metrics, so initialize here
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(builder.unsecureGrpcServer)
	grpc_prometheus.Register(builder.secureGrpcServer)
}

func (builder *RPCEngineBuilder) withRegisterRPC() {
	accessproto.RegisterAccessAPIServer(builder.unsecureGrpcServer, builder.handler)
	accessproto.RegisterAccessAPIServer(builder.secureGrpcServer, builder.handler)
}

func (builder *RPCEngineBuilder) Build() *Engine {
	builder.withRegisterRPC()
	return builder.Engine
}
