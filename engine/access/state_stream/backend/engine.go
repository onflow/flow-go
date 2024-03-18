package backend

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow/protobuf/go/flow/executiondata"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// Engine exposes the server with the state stream API.
// By default, this engine is not enabled.
// In order to run this engine a port for the GRPC server to be served on should be specified in the run config.
type Engine struct {
	*component.ComponentManager
	log     zerolog.Logger
	backend *StateStreamBackend
	config  Config
	chain   flow.Chain
	handler *Handler

	execDataCache *cache.ExecutionDataCache
	headers       storage.Headers
}

// NewEng returns a new ingress server.
func NewEng(
	log zerolog.Logger,
	config Config,
	execDataCache *cache.ExecutionDataCache,
	headers storage.Headers,
	chainID flow.ChainID,
	server *grpcserver.GrpcServer,
	backend *StateStreamBackend,
) (*Engine, error) {
	logger := log.With().Str("engine", "state_stream_rpc").Logger()

	e := &Engine{
		log:           logger,
		backend:       backend,
		headers:       headers,
		chain:         chainID.Chain(),
		config:        config,
		handler:       NewHandler(backend, chainID.Chain(), config),
		execDataCache: execDataCache,
	}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			<-server.Done()
		}).
		Build()

	executiondata.RegisterExecutionDataAPIServer(server.Server, e.handler)

	return e, nil
}
