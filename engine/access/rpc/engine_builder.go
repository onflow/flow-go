package rpc

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"

	"github.com/onflow/flow-go/access"
	legacyaccess "github.com/onflow/flow-go/access/legacy"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
)

type RPCEngineBuilder struct {
	*Engine
	handler              accessproto.AccessAPIServer // Use the parent interface instead of implementation, so that we can assign it to proxy.
	signerIndicesDecoder hotstuff.BlockSignerDecoder
}

// NewRPCEngineBuilder helps to build a new RPC engine.
func NewRPCEngineBuilder(engine *Engine) *RPCEngineBuilder {
	decoder := signature.NewNoopBlockSignerDecoder()

	// the default handler will use the engine.backend implementation
	return &RPCEngineBuilder{
		Engine:               engine,
		signerIndicesDecoder: decoder,
		handler:              access.NewHandler(engine.backend, engine.chain, access.WithBlockSignerDecoder(decoder)),
	}
}

func (builder *RPCEngineBuilder) Handler() accessproto.AccessAPIServer {
	return builder.handler
}

// WithBlockSignerDecoder specifies that signer indices in block headers should be translated
// to full node IDs with the given decoder.
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithBlockSignerDecoder(signerIndicesDecoder hotstuff.BlockSignerDecoder) *RPCEngineBuilder {
	builder.signerIndicesDecoder = signerIndicesDecoder
	return builder
}

func (builder *RPCEngineBuilder) WithNewHandler(handler accessproto.AccessAPIServer) *RPCEngineBuilder {
	builder.handler = handler
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
	accessproto.RegisterAccessAPIServer(builder.unsecureGrpcServer, builder.handler)
	accessproto.RegisterAccessAPIServer(builder.secureGrpcServer, builder.handler)
	return builder.Engine
}
