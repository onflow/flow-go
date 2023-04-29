package rpc

import (
	"fmt"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"

	"github.com/onflow/flow-go/access"
	legacyaccess "github.com/onflow/flow-go/access/legacy"
	"github.com/onflow/flow-go/consensus/hotstuff"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/module"
)

type RPCEngineBuilder struct {
	*Engine

	// optional parameters, only one can be set during build phase
	signerIndicesDecoder hotstuff.BlockSignerDecoder
	handler              accessproto.AccessAPIServer // Use the parent interface instead of implementation, so that we can assign it to proxy.
	finalizedHeaderCache *synceng.FinalizedHeaderCache
	me                   module.Local
}

// NewRPCEngineBuilder helps to build a new RPC engine.
func NewRPCEngineBuilder(engine *Engine, me module.Local) *RPCEngineBuilder {
	// the default handler will use the engine.backend implementation
	return &RPCEngineBuilder{
		Engine: engine,
		me:     me,
	}
}

func (builder *RPCEngineBuilder) Handler() accessproto.AccessAPIServer {
	return builder.handler
}

// WithBlockSignerDecoder specifies that signer indices in block headers should be translated
// to full node IDs with the given decoder.
// Caution:
// you can inject either a `BlockSignerDecoder` (via method `WithBlockSignerDecoder`)
// or an `AccessAPIServer` (via method `WithNewHandler`); but not both. If both are
// specified, the builder will error during the build step.
//
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithBlockSignerDecoder(signerIndicesDecoder hotstuff.BlockSignerDecoder) *RPCEngineBuilder {
	builder.signerIndicesDecoder = signerIndicesDecoder
	return builder
}

// WithNewHandler specifies that the given `AccessAPIServer` should be used for serving API queries.
// Caution:
// you can inject either a `BlockSignerDecoder` (via method `WithBlockSignerDecoder`)
// or an `AccessAPIServer` (via method `WithNewHandler`); but not both. If both are
// specified, the builder will error during the build step.
//
// Returns self-reference for chaining.
func (builder *RPCEngineBuilder) WithNewHandler(handler accessproto.AccessAPIServer) *RPCEngineBuilder {
	builder.handler = handler
	return builder
}

// WithFinalizedHeaderCache method specifies that the newly created `AccessAPIServer` should use
// the given `FinalizedHeaderCache` to retrieve information about the finalized block that will be included
// in the server's responses.
// Caution:
// When injecting `BlockSignerDecoder` (via the WithBlockSignerDecoder method), you must also inject
// the `FinalizedHeaderCache` or the builder will error during the build step.
//
// The method returns a self-reference for chaining.
func (builder *RPCEngineBuilder) WithFinalizedHeaderCache(cache *synceng.FinalizedHeaderCache) *RPCEngineBuilder {
	builder.finalizedHeaderCache = cache
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

func (builder *RPCEngineBuilder) Build() (*Engine, error) {
	if builder.signerIndicesDecoder != nil && builder.handler != nil {
		return nil, fmt.Errorf("only BlockSignerDecoder (via method `WithBlockSignerDecoder`) or AccessAPIServer (via method `WithNewHandler`) can be specified but not both")
	}
	handler := builder.handler
	if handler == nil {
		if builder.finalizedHeaderCache == nil {
			return nil, fmt.Errorf("FinalizedHeaderCache (via method `WithFinalizedHeaderCache`) has to be specified")
		}
		if builder.signerIndicesDecoder == nil {
			handler = access.NewHandler(builder.Engine.backend, builder.Engine.chain, builder.finalizedHeaderCache, builder.me)
		} else {
			handler = access.NewHandler(builder.Engine.backend, builder.Engine.chain, builder.finalizedHeaderCache, builder.me, access.WithBlockSignerDecoder(builder.signerIndicesDecoder))
		}
	}
	accessproto.RegisterAccessAPIServer(builder.unsecureGrpcServer, handler)
	accessproto.RegisterAccessAPIServer(builder.secureGrpcServer, handler)
	return builder.Engine, nil
}
