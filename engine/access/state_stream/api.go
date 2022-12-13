package state_stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	executiondata "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	DefaultCacheSize   = 100
	DefaultSendTimeout = 1 * time.Second
)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error)
	SubscribeExecutionData(ctx context.Context) *ExecutionDataSubscription
}

type StateStreamBackend struct {
	log           zerolog.Logger
	headers       storage.Headers
	seals         storage.Seals
	results       storage.ExecutionResults
	execDataStore execution_data.ExecutionDataStore
	execDataCache *lru.Cache
	responseCache *lru.Cache
	broadcaster   *engine.Broadcaster
	sendTimeout   time.Duration

	latestBlockCache     *LatestEntityIDCache
	signerIndicesDecoder *signature.NoopBlockSignerDecoder
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
	responseCache, err := lru.New(DefaultCacheSize)
	if err != nil {
		return nil, fmt.Errorf("could not create cache: %w", err)
	}

	return &StateStreamBackend{
		log:           log.With().Str("module", "state_stream_api").Logger(),
		headers:       headers,
		seals:         seals,
		results:       results,
		execDataStore: execDataStore,
		execDataCache: execDataCache,
		responseCache: responseCache,
		broadcaster:   broadcaster,
		sendTimeout:   DefaultSendTimeout,

		latestBlockCache:     latestBlockCache,
		signerIndicesDecoder: &signature.NoopBlockSignerDecoder{},
	}, nil
}

func (s *StateStreamBackend) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error) {
	blockExecData, err := s.getExecutionData(ctx, blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	message, err := convert.BlockExecutionDataToMessage(blockExecData)
	if err != nil {
		return nil, fmt.Errorf("could not convert execution data to entity: %w", err)
	}

	return message, nil
}

func (s *StateStreamBackend) SubscribeExecutionData(ctx context.Context) *ExecutionDataSubscription {
	notifier := s.broadcaster.Subscribe()
	lastHeight := uint64(0)

	sub := &ExecutionDataSubscription{
		ch: make(chan *executiondata.SubscribeExecutionDataResponse),
	}

	subID := unittest.GenerateRandomStringWithLen(16)
	lg := s.log.With().Str("sub_id", subID).Logger()

	lg.Debug().Msg("new execution data subscription")

	go func() {
		defer close(sub.ch)
		defer lg.Debug().Msg("finished execution data subscription")
		for {
			select {
			case <-ctx.Done():
				sub.err = fmt.Errorf("client disconnected: %w", ctx.Err())
				return
			case <-notifier.Channel():
				lg.Debug().Msg("received broadcast notification")
			}

			// send all available responses
			for {
				var err error
				var response *executiondata.SubscribeExecutionDataResponse

				if lastHeight == 0 {
					// use the latest block on the first response over the stream
					response, err = s.getResponseByBlockId(ctx, s.latestBlockCache.Get())
				} else {
					response, err = s.getResponseByHeight(ctx, lastHeight+1)
				}

				if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
					// no more blocks available
					break
				}
				if err != nil {
					sub.err = fmt.Errorf("could not get response for block %d: %w", lastHeight+1, err)
					return
				}

				lg.Debug().Msgf("sending response for %d", lastHeight+1)

				lastHeight = response.BlockHeader.Height

				select {
				case <-ctx.Done():
					sub.err = fmt.Errorf("client disconnected")
					return
				case <-time.After(s.sendTimeout):
					// bail on slow clients
					sub.err = fmt.Errorf("timeout sending response")
					return
				case sub.ch <- response:
				}
			}
		}
	}()
	return sub
}

func (s *StateStreamBackend) getExecutionData(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error) {
	if cached, ok := s.execDataCache.Get(blockID); ok {
		return cached.(*execution_data.BlockExecutionData), nil
	}

	seal, err := s.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get finalized seal for block: %w", err)
	}

	result, err := s.results.ByID(seal.ResultID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution result: %w", err)
	}

	blockExecData, err := s.execDataStore.GetExecutionData(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data: %w", err)
	}

	s.execDataCache.Add(blockID, blockExecData)

	return blockExecData, nil
}

func (s *StateStreamBackend) getResponseByBlockId(ctx context.Context, blockID flow.Identifier) (*executiondata.SubscribeExecutionDataResponse, error) {
	header, err := s.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block header for block %v: %w", blockID, err)
	}

	return s.getResponseByHeight(ctx, header.Height)
}

func (s *StateStreamBackend) getResponseByHeight(ctx context.Context, height uint64) (*executiondata.SubscribeExecutionDataResponse, error) {
	if cached, ok := s.responseCache.Get(height); ok {
		return cached.(*executiondata.SubscribeExecutionDataResponse), nil
	}

	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get block header: %w", err)
	}

	blockExecData, err := s.getExecutionData(ctx, header.ID())
	if err != nil {
		return nil, err
	}

	execData, err := convert.BlockExecutionDataToMessage(blockExecData)
	if err != nil {
		return nil, fmt.Errorf("could not convert execution data to entity: %w", err)
	}

	signerIDs, err := s.signerIndicesDecoder.DecodeSignerIDs(header)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer IDs: %w", err)
	}

	headerMsg, err := convert.BlockHeaderToMessage(header, signerIDs)
	if err != nil {
		return nil, fmt.Errorf("could not convert block header to message: %w", err)
	}

	response := &executiondata.SubscribeExecutionDataResponse{
		BlockExecutionData: execData,
		BlockHeader:        headerMsg,
	}

	s.responseCache.Add(height, response)
	// TODO: can we remove the execDataCache entry here? it may still be useful for the polling case

	return response, nil
}

type ExecutionDataSubscription struct {
	ch  chan *executiondata.SubscribeExecutionDataResponse
	err error
}

func (sub *ExecutionDataSubscription) Channel() <-chan *executiondata.SubscribeExecutionDataResponse {
	return sub.ch
}

func (sub *ExecutionDataSubscription) Err() error {
	return sub.err
}
