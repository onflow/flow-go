package rpc

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"

	"github.com/onflow/flow-go/access"
	legacyaccess "github.com/onflow/flow-go/access/legacy"
	"github.com/onflow/flow-go/apiproxy"
	"github.com/onflow/flow-go/consensus/hotstuff"
)

// NewRPCEngineBuilder helps to build a new RPC engine.
func NewRPCEngineBuilder(engine *Engine) *RPCEngineBuilder {
	return &RPCEngineBuilder{
		Engine:         engine,
		sgnIdcsDecoder: signature.NewNoopBlockSignerDecoder(),
	}
}

type RPCEngineBuilder struct {
	*Engine

	router         *apiproxy.FlowAccessAPIRouter // this is set through `WithRouting`; or nil if not explicitly specified
	sgnIdcsDecoder hotstuff.BlockSignerDecoder
}

// WithRouting specifies that the given router should be used as primary access API.
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithRouting(router *apiproxy.FlowAccessAPIRouter) *RPCEngineBuilder {
	builder.router = router
	return builder
}

// WithBlockSignerDecoder specifies that signer indices in block headers should be translated
// to full node IDs with the given decoder.
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithBlockSignerDecoder(sgnIdcsDecoder hotstuff.BlockSignerDecoder) *RPCEngineBuilder {
	builder.sgnIdcsDecoder = sgnIdcsDecoder
	return builder
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
	return builder
}

func (builder *RPCEngineBuilder) Build() *Engine {
	var localAPIServer accessproto.AccessAPIServer = access.NewHandler(builder.backend, builder.chain, access.WithBlockSignerDecoder(builder.sgnIdcsDecoder))

	if builder.router != nil {
		builder.router.SetLocalAPI(localAPIServer)
		localAPIServer = builder.router
	}

	accessproto.RegisterAccessAPIServer(builder.unsecureGrpcServer, localAPIServer)
	accessproto.RegisterAccessAPIServer(builder.secureGrpcServer, localAPIServer)

	return builder.Engine
}
