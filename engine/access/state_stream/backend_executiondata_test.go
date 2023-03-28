package state_stream

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

var testEventTypes = []flow.EventType{
	"A.0x1.Foo.Bar",
	"A.0x2.Zoo.Moo",
	"A.0x3.Goo.Hoo",
}

type BackendExecutionDataSuite struct {
	suite.Suite

	state    *protocolmock.State
	snapshot *protocolmock.Snapshot
	headers  *storagemock.Headers
	seals    *storagemock.Seals
	results  *storagemock.ExecutionResults

	bs                  blobs.Blobstore
	eds                 execution_data.ExecutionDataStore
	broadcaster         *engine.Broadcaster
	execDataDistributor *requester.ExecutionDataDistributor
	execDataCache       *herocache.Cache
	backend             *StateStreamBackend

	blocks      []*flow.Block
	blockEvents map[flow.Identifier]flow.EventsList
	execDataMap map[flow.Identifier]*execution_data.BlockExecutionDataEntity
	blockMap    map[uint64]*flow.Block
	sealMap     map[flow.Identifier]*flow.Seal
	resultMap   map[flow.Identifier]*flow.ExecutionResult
}

func TestBackendExecutionDataSuite(t *testing.T) {
	suite.Run(t, new(BackendExecutionDataSuite))
}

func (s *BackendExecutionDataSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())

	unittest.LogVerbose()
	logger := unittest.Logger()

	s.state = protocolmock.NewState(s.T())
	s.snapshot = protocolmock.NewSnapshot(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.results = storagemock.NewExecutionResults(s.T())

	s.bs = blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	s.eds = execution_data.NewExecutionDataStore(s.bs, execution_data.DefaultSerializer)

	s.broadcaster = engine.NewBroadcaster()
	s.execDataDistributor = requester.NewExecutionDataDistributor()

	s.execDataCache = herocache.NewCache(
		DefaultCacheSize,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger,
		metrics.NewNoopCollector(),
	)

	var err error
	s.backend, err = New(
		logger,
		s.state,
		s.headers,
		s.seals,
		s.results,
		s.eds,
		s.execDataCache,
		s.broadcaster,
	)
	require.NoError(s.T(), err)

	blockCount := 5
	s.execDataMap = make(map[flow.Identifier]*execution_data.BlockExecutionDataEntity, blockCount)
	s.blockEvents = make(map[flow.Identifier]flow.EventsList, blockCount)
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.sealMap = make(map[flow.Identifier]*flow.Seal, blockCount)
	s.resultMap = make(map[flow.Identifier]*flow.ExecutionResult, blockCount)
	s.blocks = make([]*flow.Block, 0, blockCount)

	// generate blockCount consecutive blocks with associated seal, result and execution data
	firstBlock := unittest.BlockFixture()
	parent := firstBlock.Header
	for i := 0; i < blockCount; i++ {
		var block *flow.Block
		if i == 0 {
			block = &firstBlock
		} else {
			block = unittest.BlockWithParentFixture(parent)
		}
		// update for next iteration
		parent = block.Header

		seal := unittest.BlockSealsFixture(1)[0]
		result := unittest.ExecutionResultFixture()
		blockEvents := unittest.BlockEventsFixture(block.Header, (i%len(testEventTypes))*3+1, testEventTypes...)
		execData := blockExecutionDataFixture(s.T(), block, blockEvents.Events)

		result.ExecutionDataID, err = s.eds.AddExecutionData(context.TODO(), execData)
		assert.NoError(s.T(), err)

		s.blocks = append(s.blocks, block)
		s.execDataMap[block.ID()] = execution_data.NewBlockExecutionDataEntity(result.ExecutionDataID, execData)
		s.blockEvents[block.ID()] = blockEvents.Events
		s.blockMap[block.Header.Height] = block
		s.sealMap[block.ID()] = seal
		s.resultMap[seal.ResultID] = result

		s.T().Logf("adding exec data for block %d %d %v => %v", i, block.Header.Height, block.ID(), result.ExecutionDataID)
	}

	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()
	s.snapshot.On("Head").Return(firstBlock.Header, nil).Maybe()

	s.seals.On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Seal {
			if seal, ok := s.sealMap[blockID]; ok {
				return seal
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			if _, ok := s.sealMap[blockID]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			if result, ok := s.resultMap[resultID]; ok {
				return result
			}
			return nil
		},
		func(resultID flow.Identifier) error {
			if _, ok := s.resultMap[resultID]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Header {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return nil
				}
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Header {
			if block, ok := s.blockMap[height]; ok {
				return block.Header
			}
			return nil
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()
}

func (s *BackendExecutionDataSuite) TestGetExecutionDataByBlockID() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := s.blocks[0]
	seal := s.sealMap[block.ID()]
	result := s.resultMap[seal.ResultID]
	execData := s.execDataMap[block.ID()]

	var err error
	s.Run("happy path TestGetExecutionDataByBlockID success", func() {
		result.ExecutionDataID, err = s.eds.AddExecutionData(ctx, execData.BlockExecutionData)
		require.NoError(s.T(), err)

		res, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID())
		assert.Equal(s.T(), execData.BlockExecutionData, res)
		assert.NoError(s.T(), err)
	})

	s.execDataCache.Clear()

	s.Run("missing exec data for TestGetExecutionDataByBlockID failure", func() {
		result.ExecutionDataID = unittest.IdentifierFixture()

		execDataRes, err := s.backend.GetExecutionDataByBlockID(ctx, block.ID())
		assert.Nil(s.T(), execDataRes)
		assert.Equal(s.T(), codes.NotFound, status.Code(err))
	})
}

func blockExecutionDataFixture(t *testing.T, block *flow.Block, events []flow.Event) *execution_data.BlockExecutionData {
	numChunks := 5
	minSerializedSize := 5 * execution_data.DefaultMaxBlobSize

	chunks := make([]*execution_data.ChunkExecutionData, numChunks)

	for i := 0; i < numChunks; i++ {
		var e flow.EventsList
		switch {
		case i >= len(events):
			e = flow.EventsList{}
		case i == numChunks-1:
			e = events[i:]
		default:
			e = flow.EventsList{events[i]}
		}
		chunks[i] = chunkExecutionDataFixture(t, uint64(minSerializedSize), e)
	}

	return &execution_data.BlockExecutionData{
		BlockID:             block.ID(),
		ChunkExecutionDatas: chunks,
	}
}

func chunkExecutionDataFixture(t *testing.T, minSerializedSize uint64, events []flow.Event) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		TrieUpdate: testutils.TrieUpdateFixture(1, 1, 8),
		Events:     events,
	}

	size := 1

	for {
		buf := &bytes.Buffer{}
		require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, ced))
		if buf.Len() >= int(minSerializedSize) {
			return ced
		}

		v := make([]byte, size)
		_, _ = rand.Read(v)

		k, err := ced.TrieUpdate.Payloads[0].Key()
		require.NoError(t, err)

		ced.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
		size *= 2
	}
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionData() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name            string
		highestBackfill int
		startBlockID    flow.Identifier
		startHeight     uint64
	}{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startBlockID:    flow.ZeroID,
			startHeight:     0,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.blocks[0].ID(),
			startHeight:     0,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			// make sure we're starting with a fresh cache
			s.execDataCache.Clear()

			s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

			for i := 0; i <= test.highestBackfill; i++ {
				s.T().Logf("backfilling block %d", i)
				execData := s.execDataMap[s.blocks[i].ID()]
				s.execDataDistributor.OnExecutionDataReceived(execData)
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeExecutionData(subCtx, test.startBlockID, test.startHeight)

			// loop over all of the blocks
			for i, b := range s.blocks {
				execData := s.execDataMap[b.ID()]
				s.T().Logf("checking block %d %v", i, b.ID())

				// simulate new exec data received
				if i > test.highestBackfill {
					s.execDataDistributor.OnExecutionDataReceived(execData)
					s.broadcaster.Publish()
				}

				// consume execution data from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					resp, ok := v.(*ExecutionDataResponse)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					assert.Equal(s.T(), b.Header.Height, resp.Height)
					assert.Equal(s.T(), execData.BlockExecutionData, resp.ExecutionData)
				}, 100*time.Millisecond, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
			}

			// make sure there are no new messages waiting
			unittest.RequireNeverReturnBefore(s.T(), func() {
				// this is a failure case. the channel should be opened with nothing waiting
				<-sub.Channel()
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")

			// stop the subscription
			subCancel()

			// ensure subscription shuts down gracefully
			unittest.RequireReturnsBefore(s.T(), func() {
				v, ok := <-sub.Channel()
				assert.Nil(s.T(), v)
				assert.False(s.T(), ok)
				assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
		})
	}
}

func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	block := unittest.BlockFixture()
	seal := unittest.BlockSealsFixture(1)[0]
	result := unittest.ExecutionResultFixture()
	execData := blockExecutionDataFixture(s.T(), &block, nil)

	result.ExecutionDataID, err = s.eds.AddExecutionData(ctx, execData)
	assert.NoError(s.T(), err)

	s.execDataMap[block.ID()] = execution_data.NewBlockExecutionDataEntity(result.ExecutionDataID, execData)
	s.blockMap[block.Header.Height] = &block
	s.sealMap[block.ID()] = seal
	s.resultMap[seal.ResultID] = result

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeExecutionData(subCtx, unittest.IdentifierFixture(), 0)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()))
	})

	// make sure we're starting with a fresh cache
	s.execDataCache.Clear()

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeExecutionData(subCtx, flow.ZeroID, block.Header.Height+10)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()))
	})
}
