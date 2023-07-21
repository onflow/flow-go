package state_stream

import (
	"fmt"
	"time"

	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	EventFilterConfig

	// ListenAddr is the address the GRPC server will listen on as host:port
	ListenAddr string

	// MaxExecutionDataMsgSize is the max message size for block execution data API
	MaxExecutionDataMsgSize uint

	// RpcMetricsEnabled specifies whether to enable the GRPC metrics
	RpcMetricsEnabled bool

	// MaxGlobalStreams defines the global max number of streams that can be open at the same time.
	MaxGlobalStreams uint32

	// ExecutionDataCacheSize is the max number of objects for the execution data cache.
	ExecutionDataCacheSize uint32

	// ClientSendTimeout is the timeout for sending a message to the client. After the timeout,
	// the stream is closed with an error.
	ClientSendTimeout time.Duration

	// ClientSendBufferSize is the size of the response buffer for sending messages to the client.
	ClientSendBufferSize uint

	// ResponseLimit is the max responses per second allowed on a stream. After exceeding the limit,
	// the stream is paused until more capacity is available. Searches of past data can be CPU
	// intensive, so this helps manage the impact.
	ResponseLimit float64
}

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

	execDataBroadcaster *engine.Broadcaster
	execDataCache       *cache.ExecutionDataCache
	headers             storage.Headers
}

// NewEng returns a new ingress server.
func NewEng(
	log zerolog.Logger,
	config Config,
	execDataStore execution_data.ExecutionDataStore,
	execDataCache *cache.ExecutionDataCache,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	chainID flow.ChainID,
	initialBlockHeight uint64,
	highestBlockHeight uint64,
	server *grpcserver.GrpcServer,
) (*Engine, error) {
	logger := log.With().Str("engine", "state_stream_rpc").Logger()

	broadcaster := engine.NewBroadcaster()

	backend, err := New(
		logger,
		config,
		state,
		headers,
		seals,
		results,
		execDataStore,
		execDataCache,
		broadcaster,
		initialBlockHeight,
		highestBlockHeight,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create state stream backend: %w", err)
	}

	e := &Engine{
		log:                 logger,
		backend:             backend,
		headers:             headers,
		chain:               chainID.Chain(),
		config:              config,
		handler:             NewHandler(backend, chainID.Chain(), config.EventFilterConfig, config.MaxGlobalStreams),
		execDataBroadcaster: broadcaster,
		execDataCache:       execDataCache,
	}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			<-server.Done()
		}).
		Build()

	access.RegisterExecutionDataAPIServer(server.Server, e.handler)

	return e, nil
}

// OnExecutionData is called to notify the engine when a new execution data is received.
// The caller must guarantee that execution data is locally available for all blocks with
// heights between the initialBlockHeight provided during startup and the block height of
// the execution data provided.
func (e *Engine) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) {
	lg := e.log.With().Hex("block_id", logging.ID(executionData.BlockID)).Logger()

	lg.Trace().Msg("received execution data")

	header, err := e.headers.ByBlockID(executionData.BlockID)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		lg.Fatal().Err(err).Msg("failed to get header for execution data")
		return
	}

	if ok := e.backend.setHighestHeight(header.Height); !ok {
		// this means that the height was lower than the current highest height
		// OnExecutionData is guaranteed by the requester to be called in order, but may be called
		// multiple times for the same block.
		lg.Debug().Msg("execution data for block already received")
		return
	}

	e.execDataBroadcaster.Publish()
}
