package backend

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	state_stream.EventFilterConfig

	// ListenAddr is the address the GRPC server will listen on as host:port
	ListenAddr string

	// MaxExecutionDataMsgSize is the max message size for block execution data API
	MaxExecutionDataMsgSize uint

	// RpcMetricsEnabled specifies whether to enable the GRPC metrics
	RpcMetricsEnabled bool

	// MaxGlobalStreams defines the global max number of streams that can be open at the same time.
	MaxGlobalStreams uint32

	// RegisterIDsRequestLimit defines the max number of register IDs that can be received in a single request.
	RegisterIDsRequestLimit uint32

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

	// HeartbeatInterval specifies the block interval at which heartbeat messages should be sent.
	HeartbeatInterval uint64
}

type GetExecutionDataFunc func(context.Context, uint64) (*execution_data.BlockExecutionDataEntity, error)

type StateStreamBackend struct {
	tracker.ExecutionDataTracker

	ExecutionDataBackend
	EventsBackend
	AccountStatusesBackend

	log                  zerolog.Logger
	state                protocol.State
	headers              storage.Headers
	seals                storage.Seals
	results              storage.ExecutionResults
	execDataStore        execution_data.ExecutionDataStore
	execDataCache        *cache.ExecutionDataCache
	registers            *execution.RegistersAsyncStore
	registerRequestLimit int
}

var _ state_stream.API = (*StateStreamBackend)(nil)

func New(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDataStore execution_data.ExecutionDataStore,
	execDataCache *cache.ExecutionDataCache,
	registers *execution.RegistersAsyncStore,
	useEventsIndex bool,
	registerIDsRequestLimit int,
	subscriptionFactory *subscription.Factory,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) (*StateStreamBackend, error) {
	logger := log.With().Str("module", "state_stream_api").Logger()

	b := &StateStreamBackend{
		ExecutionDataTracker: executionDataTracker,
		log:                  logger,
		state:                state,
		headers:              headers,
		seals:                seals,
		results:              results,
		execDataStore:        execDataStore,
		execDataCache:        execDataCache,
		registers:            registers,
		registerRequestLimit: registerIDsRequestLimit,
	}

	executionDataProvider := NewExecutionDataProvider(
		executionDataTracker,
		execDataCache,
		state,
	)

	b.ExecutionDataBackend = ExecutionDataBackend{
		log:                     logger,
		headers:                 headers,
		subscriptionFactory:     subscriptionFactory,
		executionDataTracker:    executionDataTracker,
		execDataProvider:        executionDataProvider,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
	}

	eventProvider := NewEventsProvider(
		log,
		headers,
		executionDataProvider,
		useEventsIndex,
		executionResultProvider,
		executionStateCache,
	)

	b.EventsBackend = EventsBackend{
		log:                  logger,
		subscriptionFactory:  subscriptionFactory,
		executionDataTracker: executionDataTracker,
		eventsProvider:       eventProvider,
	}

	b.AccountStatusesBackend = AccountStatusesBackend{
		log:                  logger,
		subscriptionFactory:  subscriptionFactory,
		executionDataTracker: b.ExecutionDataTracker,
		eventsProvider:       eventProvider,
	}

	return b, nil
}

// GetRegisterValues returns the register values for the given register IDs at the given block height.
func (b *StateStreamBackend) GetRegisterValues(ids flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	if len(ids) > b.registerRequestLimit {
		return nil, status.Errorf(codes.InvalidArgument, "number of register IDs exceeds limit of %d", b.registerRequestLimit)
	}

	values, err := b.registers.RegisterValues(ids, height)
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) {
			return nil, status.Errorf(codes.OutOfRange, "register values for block %d is not available", height)
		}
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "register values for block %d not found", height)
		}
		return nil, err
	}

	return values, nil
}
