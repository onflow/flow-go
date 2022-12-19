package state_stream

import (
	"context"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

const (
	DefaultCacheSize = 100

	// DefaultSendTimeout is the default timeout for sending a message to the client. After the timeout
	// expires, the connection is closed.
	DefaultSendTimeout = 30 * time.Second
)

type GetExecutionDataFunc func(context.Context, flow.Identifier) (*execution_data.BlockExecutionData, error)
type GetStartHeightFunc func(flow.Identifier, uint64) (uint64, error)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error)
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) Subscription
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription
}

type StateStreamBackend struct {
	ExecutionDataBackend
	EventsBackend

	log              zerolog.Logger
	headers          storage.Headers
	seals            storage.Seals
	results          storage.ExecutionResults
	execDataStore    execution_data.ExecutionDataStore
	execDataCache    *lru.Cache
	broadcaster      *engine.Broadcaster
	sendTimeout      time.Duration
	latestBlockCache *LatestEntityIDCache
}

func New(
	log zerolog.Logger,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDataStore execution_data.ExecutionDataStore,
	execDataCache *lru.Cache,
	broadcaster *engine.Broadcaster,
	latestBlockCache *LatestEntityIDCache,
) (*StateStreamBackend, error) {
	logger := log.With().Str("module", "state_stream_api").Logger()

	b := &StateStreamBackend{
		log:           logger,
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

func (b *StateStreamBackend) getExecutionData(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error) {
	if cached, ok := b.execDataCache.Get(blockID); ok {
		return cached.(*execution_data.BlockExecutionData), nil
	}

	seal, err := b.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get finalized seal for block: %w", err)
	}

	result, err := b.results.ByID(seal.ResultID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution result: %w", err)
	}

	blockExecData, err := b.execDataStore.GetExecutionData(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data: %w", err)
	}

	b.execDataCache.Add(blockID, blockExecData)

	return blockExecData, nil
}

func (b *StateStreamBackend) getStartHeight(startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
	// first, if a start block ID is provided, use that
	// invalid or missing block IDs will result in an error
	if startBlockID != flow.ZeroID {
		header, err := b.headers.ByBlockID(startBlockID)
		if err != nil {
			return 0, fmt.Errorf("could not get header for block %v: %w", startBlockID, err)
		}
		return header.Height, nil
	}

	// next, if the start height is provided, use that
	// heights that are in the future or before the root block will result in an error
	if startHeight > 0 {
		header, err := b.headers.ByHeight(startHeight)
		if err != nil {
			return 0, fmt.Errorf("could not get header for height %d: %w", startHeight, err)
		}
		return header.Height, nil
	}

	// finally, if no start block ID or height is provided, use the latest block
	header, err := b.headers.ByBlockID(b.latestBlockCache.Get())
	if err != nil {
		// this should never happen and would indicate there's an issue with the protocol state,
		// but do not crash the node as a result of an external request.
		return 0, fmt.Errorf("could not get header for latest block: %w", err)
	}

	return header.Height, nil
}
