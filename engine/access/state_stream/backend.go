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
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// DefaultMaxGlobalStreams defines the default max number of streams that can be open at the same time.
	DefaultMaxGlobalStreams = 1000

	// DefaultCacheSize defines the default max number of objects for the execution data cache.
	DefaultCacheSize = 100

	// DefaultSendTimeout is the default timeout for sending a message to the client. After the timeout
	// expires, the connection is closed.
	DefaultSendTimeout = 30 * time.Second

	// DefaultThrottleDelay is the default delay to inject between searching each block to throttle scans
	DefaultThrottleDelay = time.Duration(0)
)

type GetExecutionDataFunc func(context.Context, flow.Identifier) (*execution_data.BlockExecutionDataEntity, error)
type GetStartHeightFunc func(flow.Identifier, uint64) (uint64, error)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error)
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) Subscription
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription
}

type StateStreamBackend struct {
	ExecutionDataBackend
	EventsBackend

	log           zerolog.Logger
	state         protocol.State
	headers       storage.Headers
	seals         storage.Seals
	results       storage.ExecutionResults
	execDataStore execution_data.ExecutionDataStore
	execDataCache *herocache.BlockExecutionData
	broadcaster   *engine.Broadcaster
}

func New(
	log zerolog.Logger,
	config Config,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDataStore execution_data.ExecutionDataStore,
	execDataCache *herocache.BlockExecutionData,
	broadcaster *engine.Broadcaster,
) (*StateStreamBackend, error) {
	logger := log.With().Str("module", "state_stream_api").Logger()

	b := &StateStreamBackend{
		log:           logger,
		state:         state,
		headers:       headers,
		seals:         seals,
		results:       results,
		execDataStore: execDataStore,
		execDataCache: execDataCache,
		broadcaster:   broadcaster,
	}

	b.ExecutionDataBackend = ExecutionDataBackend{
		log:              logger,
		headers:          headers,
		broadcaster:      broadcaster,
		sendTimeout:      config.ClientSendTimeout,
		throttleDelay:    config.ThrottleDelay,
		sendBufferSize:   int(config.ClientSendBufferSize),
		getExecutionData: b.getExecutionData,
		getStartHeight:   b.getStartHeight,
	}

	b.EventsBackend = EventsBackend{
		log:              logger,
		headers:          headers,
		broadcaster:      broadcaster,
		sendTimeout:      config.ClientSendTimeout,
		throttleDelay:    config.ThrottleDelay,
		sendBufferSize:   int(config.ClientSendBufferSize),
		getExecutionData: b.getExecutionData,
		getStartHeight:   b.getStartHeight,
	}

	return b, nil
}

func (b *StateStreamBackend) getExecutionData(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error) {
	if cached, ok := b.execDataCache.ByID(blockID); ok {
		b.log.Trace().
			Hex("block_id", logging.ID(blockID)).
			Msg("execution data cache hit")
		return cached, nil
	}
	b.log.Trace().
		Hex("block_id", logging.ID(blockID)).
		Msg("execution data cache miss")

	seal, err := b.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get finalized seal for block: %w", err)
	}

	result, err := b.results.ByID(seal.ResultID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution result (id: %s): %w", seal.ResultID, err)
	}

	execData, err := b.execDataStore.GetExecutionData(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data (id: %s): %w", result.ExecutionDataID, err)
	}

	blockExecData := execution_data.NewBlockExecutionDataEntity(result.ExecutionDataID, execData)

	b.execDataCache.Add(blockExecData)

	return blockExecData, nil
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

	// first, if a start block ID is provided, use that
	// invalid or missing block IDs will result in an error
	if startBlockID != flow.ZeroID {
		header, err := b.headers.ByBlockID(startBlockID)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
		}
		return header.Height, nil
	}

	// next, if the start height is provided, use that
	// heights that are in the future or before the root block will result in an error
	if startHeight > 0 {
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
