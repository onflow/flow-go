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
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	DefaultCacheSize = 100

	// DefaultSendTimeout is the default timeout for sending a message to the client. After the timeout
	// expires, the connection is closed.
	DefaultSendTimeout = 30 * time.Second
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
	execDataCache *herocache.Cache
	broadcaster   *engine.Broadcaster
	sendTimeout   time.Duration
}

func New(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDataStore execution_data.ExecutionDataStore,
	execDataCache *herocache.Cache,
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
		sendTimeout:   DefaultSendTimeout,
	}

	b.ExecutionDataBackend = ExecutionDataBackend{
		log:              logger,
		headers:          headers,
		broadcaster:      broadcaster,
		sendTimeout:      DefaultSendTimeout,
		getExecutionData: b.getExecutionData,
		getStartHeight:   b.getStartHeight,
	}

	b.EventsBackend = EventsBackend{
		log:              logger,
		headers:          headers,
		broadcaster:      broadcaster,
		sendTimeout:      DefaultSendTimeout,
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
		return cached.(*execution_data.BlockExecutionDataEntity), nil
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
		return nil, fmt.Errorf("could not get execution result: %w", err)
	}

	execData, err := b.execDataStore.GetExecutionData(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data: %w", err)
	}

	blockExecData := execution_data.NewBlockExecutionDataEntity(result.ExecutionDataID, execData)

	b.execDataCache.Add(blockID, blockExecData)

	return blockExecData, nil
}

// getStartHeight returns the start height to use when searching
// The height is chosen using the following priority order:
// 1. startBlockID
// 2. startHeight
// 3. the latest sealed block
// If a block is provided and does not exist, an error is returned
func (b *StateStreamBackend) getStartHeight(startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
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
