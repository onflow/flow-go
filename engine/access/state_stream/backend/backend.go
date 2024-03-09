package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
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
type GetStartHeightFunc func(flow.Identifier, uint64) (uint64, error)

type StateStreamBackend struct {
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
	broadcaster          *engine.Broadcaster
	rootBlockHeight      uint64
	rootBlockID          flow.Identifier
	registers            *execution.RegistersAsyncStore
	indexReporter        state_synchronization.IndexReporter
	registerRequestLimit int
	useIndex             bool

	// highestHeight contains the highest consecutive block height for which we have received a
	// new Execution Data notification.
	highestHeight counters.StrictMonotonousCounter
}

func New(
	log zerolog.Logger,
	config Config,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDataStore execution_data.ExecutionDataStore,
	execDataCache *cache.ExecutionDataCache,
	broadcaster *engine.Broadcaster,
	rootHeight uint64,
	highestAvailableHeight uint64,
	registers *execution.RegistersAsyncStore,
	eventsIndex *backend.EventsIndex,
	useEventsIndex bool,
) (*StateStreamBackend, error) {
	logger := log.With().Str("module", "state_stream_api").Logger()

	// cache the root block height and ID for runtime lookups.
	rootBlockID, err := headers.BlockIDByHeight(rootHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get root block ID: %w", err)
	}

	b := &StateStreamBackend{
		log:                  logger,
		state:                state,
		headers:              headers,
		seals:                seals,
		results:              results,
		execDataStore:        execDataStore,
		execDataCache:        execDataCache,
		broadcaster:          broadcaster,
		rootBlockHeight:      rootHeight,
		rootBlockID:          rootBlockID,
		registers:            registers,
		indexReporter:        eventsIndex,
		registerRequestLimit: int(config.RegisterIDsRequestLimit),
		highestHeight:        counters.NewMonotonousCounter(highestAvailableHeight),
		useIndex:             useEventsIndex,
	}

	b.ExecutionDataBackend = ExecutionDataBackend{
		log:              logger,
		headers:          headers,
		broadcaster:      broadcaster,
		sendTimeout:      config.ClientSendTimeout,
		responseLimit:    config.ResponseLimit,
		sendBufferSize:   int(config.ClientSendBufferSize),
		getExecutionData: b.getExecutionData,
		getStartHeight:   b.getStartHeight,
	}

	eventsRetriever := EventsRetriever{
		log:              logger,
		headers:          headers,
		getExecutionData: b.getExecutionData,
		useEventsIndex:   useEventsIndex,
		eventsIndex:      eventsIndex,
	}

	b.EventsBackend = EventsBackend{
		log:             logger,
		broadcaster:     broadcaster,
		sendTimeout:     config.ClientSendTimeout,
		responseLimit:   config.ResponseLimit,
		sendBufferSize:  int(config.ClientSendBufferSize),
		getStartHeight:  b.getStartHeight,
		eventsRetriever: eventsRetriever,
	}

	b.AccountStatusesBackend = AccountStatusesBackend{
		log:             logger,
		broadcaster:     broadcaster,
		sendTimeout:     config.ClientSendTimeout,
		responseLimit:   config.ResponseLimit,
		sendBufferSize:  int(config.ClientSendBufferSize),
		getStartHeight:  b.getStartHeight,
		eventsRetriever: eventsRetriever,
	}

	return b, nil
}

// getExecutionData returns the execution data for the given block height.
// Expected errors during normal operation:
// - storage.ErrNotFound or execution_data.BlobNotFoundError: execution data for the given block height is not available.
func (b *StateStreamBackend) getExecutionData(ctx context.Context, height uint64) (*execution_data.BlockExecutionDataEntity, error) {
	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all streams.
	if height > b.highestHeight.Value() {
		return nil, fmt.Errorf("execution data for block %d is not available yet: %w", height, storage.ErrNotFound)
	}

	execData, err := b.execDataCache.ByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	return execData, nil
}

// getStartHeight returns the start height to use when searching.
// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
// If a block is provided and does not exist, a NotFound error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
func (b *StateStreamBackend) getStartHeight(startBlockID flow.Identifier, startHeight uint64) (height uint64, err error) {
	// make sure only one of start block ID and start height is provided
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	// ensure that the resolved start height is available
	defer func() {
		if err == nil {
			height, err = b.checkStartHeight(height)
		}
	}()

	if startBlockID != flow.ZeroID {
		return b.startHeightFromBlockID(startBlockID)
	}

	if startHeight > 0 {
		return b.startHeightFromHeight(startHeight)
	}

	// if no start block was provided, use the latest sealed block
	header, err := b.state.Sealed().Head()
	if err != nil {
		return 0, status.Errorf(codes.Internal, "could not get latest sealed block: %v", err)
	}
	return header.Height, nil
}

func (b *StateStreamBackend) startHeightFromBlockID(startBlockID flow.Identifier) (uint64, error) {
	header, err := b.headers.ByBlockID(startBlockID)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
	}
	return header.Height, nil
}

func (b *StateStreamBackend) startHeightFromHeight(startHeight uint64) (uint64, error) {
	if startHeight < b.rootBlockHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", b.rootBlockHeight)
	}

	header, err := b.headers.ByHeight(startHeight)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
	}
	return header.Height, nil
}

func (b *StateStreamBackend) checkStartHeight(height uint64) (uint64, error) {
	// if the start block is the root block, there will not be an execution data. skip it and
	// begin from the next block.
	if height == b.rootBlockHeight {
		height = b.rootBlockHeight + 1
	}

	if !b.useIndex {
		return height, nil
	}

	lowestHeight, highestHeight, err := b.getIndexerHeights()
	if err != nil {
		return 0, err
	}

	if height < lowestHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height %d is lower than lowest indexed height %d", height, lowestHeight)
	}

	if height > highestHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height %d is higher than highest indexed height %d", height, highestHeight)
	}

	return height, nil
}

// getIndexerHeights returns the lowest and highest indexed block heights
// Expected errors during normal operation:
// - codes.FailedPrecondition: if the index reporter is not ready yet.
// - codes.Internal: if there was any other error getting the heights.
func (b *StateStreamBackend) getIndexerHeights() (uint64, uint64, error) {
	lowestHeight, err := b.indexReporter.LowestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get lowest indexed height", codes.Internal)
	}

	highestHeight, err := b.indexReporter.HighestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get highest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get highest indexed height", codes.Internal)
	}

	return lowestHeight, highestHeight, nil
}

// setHighestHeight sets the highest height for which execution data is available.
func (b *StateStreamBackend) setHighestHeight(height uint64) bool {
	return b.highestHeight.Set(height)
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
