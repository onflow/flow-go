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

// WithLegacy specifies that a legacy access API should be instantiated
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithLegacy() *RPCEngineBuilder {
	// Register legacy gRPC handlers for backwards compatibility, to be removed at a later date
	legacyaccessproto.RegisterAccessAPIServer(
		builder.unsecureGrpcServer,
		legacyaccess.NewHandler(builder.backend, builder.chain),
	)
	legacyaccessproto.RegisterAccessAPIServer(
		builder.secureGrpcServer,
		legacyaccess.NewHandler(builder.backend, builder.chain),
	)
	return builder
}

// WithMetrics specifies the metrics should be collected.
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithMetrics() *RPCEngineBuilder {
	// Not interested in legacy metrics, so initialize here
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(builder.unsecureGrpcServer)
	grpc_prometheus.Register(builder.secureGrpcServer)

func (builder *RPCEngineBuilder) withRegisterRPC() {
	accessproto.RegisterAccessAPIServer(builder.unsecureGrpcServer, builder.handler)
	accessproto.RegisterAccessAPIServer(builder.secureGrpcServer, builder.handler)
}

func (builder *RPCEngineBuilder) Build() *Engine {
	var localAPIServer accessproto.AccessAPIServer = access.NewHandler(builder.backend, builder.chain, access.WithBlockSignerDecoder(builder.signerIndicesDecoder))

	if builder.router != nil {
		builder.router.SetLocalAPI(localAPIServer)
		localAPIServer = builder.router
	}

	accessproto.RegisterAccessAPIServer(builder.unsecureGrpcServer, localAPIServer)
	accessproto.RegisterAccessAPIServer(builder.secureGrpcServer, localAPIServer)

	return builder.Engine
}
