package state_stream

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultMaxGlobalStreams defines the default max number of streams that can be open at the same time.
	DefaultMaxGlobalStreams = 1000

	// DefaultCacheSize defines the default max number of objects for the execution data cache.
	DefaultCacheSize = 100

	// DefaultSendTimeout is the default timeout for sending a message to the client. After the timeout
	// expires, the connection is closed.
	DefaultSendTimeout = 30 * time.Second

	// DefaultResponseLimit is default max responses per second allowed on a stream. After exceeding
	// the limit, the stream is paused until more capacity is available.
	DefaultResponseLimit = float64(0)
)

type GetExecutionDataFunc func(context.Context, uint64) (*execution_data.BlockExecutionDataEntity, error)
type GetStartHeightFunc func(flow.Identifier, uint64) (uint64, error)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error)
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) Subscription
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription
}

type StateStreamBackend struct {
	ExecutionDataBackend
	EventsBackend

	log             zerolog.Logger
	state           protocol.State
	headers         storage.Headers
	seals           storage.Seals
	results         storage.ExecutionResults
	execDataStore   execution_data.ExecutionDataStore
	execDataCache   *cache.ExecutionDataCache
	broadcaster     *engine.Broadcaster
	rootBlockHeight uint64
	rootBlockID     flow.Identifier

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
) (*StateStreamBackend, error) {
	logger := log.With().Str("module", "state_stream_api").Logger()

	// cache the root block height and ID for runtime lookups.
	rootBlockID, err := headers.BlockIDByHeight(rootHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get root block ID: %w", err)
	}

	b := &StateStreamBackend{
		log:             logger,
		state:           state,
		headers:         headers,
		seals:           seals,
		results:         results,
		execDataStore:   execDataStore,
		execDataCache:   execDataCache,
		broadcaster:     broadcaster,
		rootBlockHeight: rootHeight,
		rootBlockID:     rootBlockID,
		highestHeight:   counters.NewMonotonousCounter(highestAvailableHeight),
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

	b.EventsBackend = EventsBackend{
		log:              logger,
		broadcaster:      broadcaster,
		sendTimeout:      config.ClientSendTimeout,
		responseLimit:    config.ResponseLimit,
		sendBufferSize:   int(config.ClientSendBufferSize),
		getExecutionData: b.getExecutionData,
		getStartHeight:   b.getStartHeight,
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
func (b *StateStreamBackend) getStartHeight(startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
	// make sure only one of start block ID and start height is provided
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	// if the start block is the root block, there will not be an execution data. skip it and
	// begin from the next block.
	// Note: we can skip the block lookup since it was already done in the constructor
	if startBlockID == b.rootBlockID || startHeight == b.rootBlockHeight {
		return b.rootBlockHeight + 1, nil
	}

	// invalid or missing block IDs will result in an error
	if startBlockID != flow.ZeroID {
		header, err := b.headers.ByBlockID(startBlockID)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
		}
		return header.Height, nil
	}

	// heights that have not been indexed yet will result in an error
	if startHeight > 0 {
		if startHeight < b.rootBlockHeight {
			return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", b.rootBlockHeight)
		}

		header, err := b.headers.ByHeight(startHeight)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
		}
		return header.Height, nil
	}

	// if no start block was provided, use the latest sealed block
	header, err := b.state.Sealed().Head()
	if err != nil {
		return 0, status.Errorf(codes.Internal, "could not get latest sealed block: %v", err)
	}
	return header.Height, nil
}

// SetHighestHeight sets the highest height for which execution data is available.
func (b *StateStreamBackend) setHighestHeight(height uint64) bool {
	return b.highestHeight.Set(height)
}
