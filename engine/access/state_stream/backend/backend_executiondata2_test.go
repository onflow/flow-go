package backend

import (
	"context"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	trackermock "github.com/onflow/flow-go/engine/access/subscription/tracker/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/execution_result"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type BackendExecutionDataSuite2 struct {
	suite.Suite

	fixtureGenerator *fixtures.GeneratorSuite

	log      zerolog.Logger
	state    *protocolmock.State
	params   *protocolmock.Params
	snapshot *protocolmock.Snapshot
	headers  *storagemock.Headers
	receipts *storagemock.ExecutionReceipts

	// execution data stuff
	executionDataBroadcaster *engine.Broadcaster
	subscriptionFactory      *subscription.SubscriptionHandler
	executionDataTracker     *trackermock.ExecutionDataTracker
	executionDataTrackerReal tracker.ExecutionDataTracker

	// optimistic sync stuff
	executionResultProvider     *osyncmock.ExecutionResultInfoProvider
	executionResultProviderReal optimistic_sync.ExecutionResultInfoProvider
	executionStateCache         *osyncmock.ExecutionStateCache
	executionDataSnapshot       *osyncmock.Snapshot
	criteria                    optimistic_sync.Criteria

	// data for the tests
	sporkRootBlock *flow.Block

	blocks                 []*flow.Block
	blocksHeightToBlockMap map[uint64]*flow.Block
	blocksIDToBlockMap     map[flow.Identifier]*flow.Block

	executionDataList         []*execution_data.BlockExecutionData
	blockIDToExecutionDataMap map[flow.Identifier]*execution_data.BlockExecutionData

	executionResults []*flow.ExecutionResult

	// execution node configuration
	fixedExecutionNodes       flow.IdentityList
	preferredExecutionNodeIDs flow.IdentifierList
}

func TestBackendExecutionDataSuite2(t *testing.T) {
	suite.Run(t, new(BackendExecutionDataSuite2))
}

func (s *BackendExecutionDataSuite2) SetupTest() {
	s.fixtureGenerator = fixtures.NewGeneratorSuite(
		fixtures.WithChainID(flow.Testnet),
		fixtures.WithSeed(42),
	)

	// blocks and execution data for the tests
	s.blocks = s.fixtureGenerator.Blocks().List(5)
	s.sporkRootBlock = s.blocks[0]
	s.blocksHeightToBlockMap = make(map[uint64]*flow.Block)
	s.blocksIDToBlockMap = make(map[flow.Identifier]*flow.Block)
	s.executionDataList = make([]*execution_data.BlockExecutionData, len(s.blocks))
	s.blockIDToExecutionDataMap = make(map[flow.Identifier]*execution_data.BlockExecutionData)
	s.executionResults = make([]*flow.ExecutionResult, len(s.blocks))

	for i, block := range s.blocks {
		s.blocksHeightToBlockMap[block.Height] = block
		s.blocksIDToBlockMap[block.ID()] = block

		execData := s.fixtureGenerator.BlockExecutionDatas().Fixture(
			fixtures.BlockExecutionData.WithBlockID(block.ID()),
		)
		if block.ID() == s.sporkRootBlock.ID() {
			// sport root block doesn't have chunks of execution data
			execData.ChunkExecutionDatas = nil
		}

		s.executionDataList[i] = execData
		s.blockIDToExecutionDataMap[block.ID()] = execData
	}

	s.fixedExecutionNodes = unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	s.preferredExecutionNodeIDs = flow.IdentifierList{} // must be subset of required
	if len(s.fixedExecutionNodes.NodeIDs()) > 0 {
		s.preferredExecutionNodeIDs = append(s.preferredExecutionNodeIDs, s.fixedExecutionNodes.NodeIDs()[0])
	}

	require.GreaterOrEqual(s.T(), 2, len(s.fixedExecutionNodes))
	s.receipts = storagemock.NewExecutionReceipts(s.T())

	// build a coherent execution-fork (single path) across blocks by
	// linking ExecutionResult.PreviousResultID between consecutive blocks.
	var prevResultID flow.Identifier
	for i, block := range s.blocks {
		receipt1 := unittest.ReceiptForBlockFixture(block)
		receipt2 := unittest.ReceiptForBlockFixture(block)

		// make both executors agree on the same execution result for this block
		receipt2.ExecutionResult = receipt1.ExecutionResult

		// link this block's result to the previous block's result to form a fork (chain)
		if i > 0 {
			receipt1.ExecutionResult.PreviousResultID = prevResultID
		}
		prevResultID = receipt1.ExecutionResult.ID()

		receipt1.ExecutorID = s.fixedExecutionNodes[0].NodeID
		receipt2.ExecutorID = s.fixedExecutionNodes[1].NodeID

		// store execution result for this block (shared by both receipts)
		s.executionResults[i] = &receipt1.ExecutionResult

		receipts := flow.ExecutionReceiptList{receipt1, receipt2}
		s.receipts.
			On("ByBlockID", block.ID()).
			Return(receipts, nil).
			Maybe()
	}

	s.log = unittest.Logger()
	s.snapshot = protocolmock.NewSnapshot(s.T())
	s.snapshot.On("Head").Return(s.blocks[0].ToHeader(), nil).Maybe()
	s.snapshot.On("SealedResult").Return(s.executionResults[0], nil, nil).Maybe() // seal is not used
	s.snapshot.On("Identities", mock.Anything).Return(s.fixedExecutionNodes, nil).Maybe()

	s.params = protocolmock.NewParams(s.T())
	s.params.On("SporkRootBlockHeight").Return(s.sporkRootBlock.Height, nil).Maybe()
	s.params.On("SporkRootBlock").Return(s.sporkRootBlock, nil).Maybe()

	s.state = protocolmock.NewState(s.T())
	s.state.On("AtBlockID", mock.Anything).Return(s.snapshot, nil).Maybe()
	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()
	s.state.On("Params").Return(s.params).Maybe()
	s.state.On("Final").Return(s.snapshot, nil).Maybe()

	s.headers = storagemock.NewHeaders(s.T())

	// execution data stuff
	s.executionDataBroadcaster = engine.NewBroadcaster()
	s.executionDataTrackerReal = tracker.NewExecutionDataTracker(
		s.log,
		s.state,
		s.sporkRootBlock.Height,
		s.headers,
		s.executionDataBroadcaster,
	)

	s.executionDataTracker = trackermock.NewExecutionDataTracker(s.T())

	s.subscriptionFactory = subscription.NewSubscriptionHandler(
		s.log,
		s.executionDataBroadcaster,
		subscription.DefaultSendTimeout,
		subscription.DefaultResponseLimit,
		subscription.DefaultSendBufferSize,
	)

	// optimistic sync stuff
	s.executionResultProvider = osyncmock.NewExecutionResultInfoProvider(s.T())
	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
	s.criteria = optimistic_sync.DefaultCriteria

	executionNodeSelector := execution_result.NewExecutionNodeSelector(
		s.preferredExecutionNodeIDs,
		s.fixedExecutionNodes.NodeIDs(),
	)

	s.executionResultProviderReal = execution_result.NewExecutionResultInfoProvider(
		s.log,
		s.state,
		s.receipts,
		executionNodeSelector,
		s.criteria,
	)
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionData() {
	tests := []struct {
		name             string
		backfilledBlocks int
		startID          flow.Identifier
		startHeight      uint64
		mockState        func(s *BackendExecutionDataSuite2)
	}{
		{
			name:             "no backfill",
			backfilledBlocks: 0,
			startID:          s.sporkRootBlock.ID(),
			startHeight:      0, // either ID or height must be provided
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeight", mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("uint64")).
					Return(
						func(ctx context.Context, ID flow.Identifier, height uint64) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeight(ctx, ID, height)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByBlockID", mock.Anything).
					Return(func(ID flow.Identifier) (*flow.Header, error) {
						block, ok := s.blocksIDToBlockMap[ID]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					})
			},
		},
		{
			name:             "partial backfill",
			backfilledBlocks: 2,
			startID:          s.sporkRootBlock.ID(),
			startHeight:      0, // either ID or height must be provided
			mockState: func(s *BackendExecutionDataSuite2) {
				require.Greater(s.T(), len(s.blocks), 2) // ensure there are enough blocks for partial backfill

				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeight", mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("uint64")).
					Return(
						func(ctx context.Context, ID flow.Identifier, height uint64) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeight(ctx, ID, height)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByBlockID", mock.Anything).
					Return(func(ID flow.Identifier) (*flow.Header, error) {
						block, ok := s.blocksIDToBlockMap[ID]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					})
			},
		},
		{
			name:             "full backfill",
			backfilledBlocks: len(s.blocks),
			startID:          s.sporkRootBlock.ID(),
			startHeight:      0, // either ID or height must be provided
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeight", mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("uint64")).
					Return(
						func(ctx context.Context, ID flow.Identifier, height uint64) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeight(ctx, ID, height)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByBlockID", mock.Anything).
					Return(func(ID flow.Identifier) (*flow.Header, error) {
						block, ok := s.blocksIDToBlockMap[ID]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					})
			},
		},
	}

	backend := s.createExecutionDataBackend()
	firstExpectedBlockHeight := s.sporkRootBlock.Height

	for _, test := range tests {
		s.Run(test.name, func() {
			test.mockState(s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := backend.SubscribeExecutionData(
				ctx,
				test.startID,
				test.startHeight,
				s.criteria,
			)

			wg := &sync.WaitGroup{}

			wg.Add(2)
			go s.writer(wg, cancel, test.backfilledBlocks)
			go s.reader(wg, sub, firstExpectedBlockHeight, context.Canceled)

			wg.Wait()
		})
	}
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromStartHeight() {
	tests := []struct {
		name             string
		backfilledBlocks int
		startHeight      uint64
		mockState        func(s *BackendExecutionDataSuite2)
	}{
		{
			name:             "no backfill",
			backfilledBlocks: 0,
			startHeight:      s.sporkRootBlock.Height,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromHeight", mock.AnythingOfType("uint64")).
					Return(
						func(startHeight uint64) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByHeight", mock.AnythingOfType("uint64")).
					Return(func(height uint64) (*flow.Header, error) {
						block, ok := s.blocksHeightToBlockMap[height]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					}).
					Once()
			},
		},
		{
			name:             "partial backfill",
			backfilledBlocks: 2,
			startHeight:      s.sporkRootBlock.Height,
			mockState: func(s *BackendExecutionDataSuite2) {
				require.Greater(s.T(), len(s.blocks), 2) // make sure there are enough blocks for partial backfill

				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromHeight", mock.AnythingOfType("uint64")).
					Return(
						func(startHeight uint64) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByHeight", mock.AnythingOfType("uint64")).
					Return(func(height uint64) (*flow.Header, error) {
						block, ok := s.blocksHeightToBlockMap[height]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					}).
					Once()
			},
		},
		{
			name:             "full backfill",
			backfilledBlocks: len(s.blocks),
			startHeight:      s.sporkRootBlock.Height,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromHeight", mock.AnythingOfType("uint64")).
					Return(
						func(startHeight uint64) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromHeight(startHeight)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByHeight", mock.AnythingOfType("uint64")).
					Return(func(height uint64) (*flow.Header, error) {
						block, ok := s.blocksHeightToBlockMap[height]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					}).
					Once()
			},
		},
	}

	backend := s.createExecutionDataBackend()
	firstExpectedBlockHeight := s.sporkRootBlock.Height

	for _, test := range tests {
		s.Run(test.name, func() {
			test.mockState(s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := backend.SubscribeExecutionDataFromStartBlockHeight(
				ctx,
				test.startHeight,
				s.criteria,
			)

			wg := &sync.WaitGroup{}
			wg.Add(2)
			go s.writer(wg, cancel, test.backfilledBlocks)
			go s.reader(wg, sub, firstExpectedBlockHeight, context.Canceled)

			wg.Wait()
		})
	}
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromStartID() {
	tests := []struct {
		name             string
		backfilledBlocks int
		startID          flow.Identifier
		mockState        func(s *BackendExecutionDataSuite2)
	}{
		{
			name:             "no backfill",
			backfilledBlocks: 0,
			startID:          s.sporkRootBlock.ID(),
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromBlockID", mock.AnythingOfType("flow.Identifier")).
					Return(
						func(ID flow.Identifier) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromBlockID(ID)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByBlockID", mock.Anything).
					Return(func(ID flow.Identifier) (*flow.Header, error) {
						block, ok := s.blocksIDToBlockMap[ID]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					})
			},
		},
		{
			name:             "partial backfill",
			backfilledBlocks: 2,
			startID:          s.sporkRootBlock.ID(),
			mockState: func(s *BackendExecutionDataSuite2) {
				require.Greater(s.T(), len(s.blocks), 2) // ensure there are enough blocks

				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromBlockID", mock.AnythingOfType("flow.Identifier")).
					Return(
						func(ID flow.Identifier) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromBlockID(ID)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByBlockID", mock.Anything).
					Return(func(ID flow.Identifier) (*flow.Header, error) {
						block, ok := s.blocksIDToBlockMap[ID]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					})
			},
		},
		{
			name:             "full backfill",
			backfilledBlocks: len(s.blocks),
			startID:          s.sporkRootBlock.ID(),
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromBlockID", mock.AnythingOfType("flow.Identifier")).
					Return(
						func(ID flow.Identifier) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromBlockID(ID)
						},
						nil,
					).
					Once()

				s.headers.
					On("ByBlockID", mock.Anything).
					Return(func(ID flow.Identifier) (*flow.Header, error) {
						block, ok := s.blocksIDToBlockMap[ID]
						if !ok {
							return nil, storage.ErrNotFound
						}
						return block.ToHeader(), nil
					})
			},
		},
	}

	backend := s.createExecutionDataBackend()
	firstExpectedBlockHeight := s.sporkRootBlock.Height

	for _, test := range tests {
		s.Run(test.name, func() {
			test.mockState(s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := backend.SubscribeExecutionDataFromStartBlockID(
				ctx,
				test.startID,
				s.criteria,
			)

			wg := &sync.WaitGroup{}

			wg.Add(2)
			go s.writer(wg, cancel, test.backfilledBlocks)
			go s.reader(wg, sub, firstExpectedBlockHeight, context.Canceled)

			wg.Wait()
		})
	}
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromLatest() {
	tests := []struct {
		name             string
		backfilledBlocks int
		mockState        func(s *BackendExecutionDataSuite2)
	}{
		{
			name:             "no backfill",
			backfilledBlocks: 0,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromLatest", mock.Anything).
					Return(
						func(ctx context.Context) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
						},
						nil,
					).
					Once()
			},
		},
		{
			name:             "partial backfill",
			backfilledBlocks: 2,
			mockState: func(s *BackendExecutionDataSuite2) {
				require.Greater(s.T(), len(s.blocks), 2) // ensure enough blocks for partial backfill

				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromLatest", mock.Anything).
					Return(
						func(ctx context.Context) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
						},
						nil,
					).
					Once()
			},
		},
		{
			name:             "full backfill",
			backfilledBlocks: len(s.blocks),
			mockState: func(s *BackendExecutionDataSuite2) {
				s.mockDataProviderFuncState()

				s.executionDataTracker.
					On("GetStartHeightFromLatest", mock.Anything).
					Return(
						func(ctx context.Context) (uint64, error) {
							return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
						},
						nil,
					).
					Once()
			},
		},
	}

	backend := s.createExecutionDataBackend()
	firstExpectedBlockHeight := s.sporkRootBlock.Height

	for _, test := range tests {
		s.Run(test.name, func() {
			test.mockState(s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := backend.SubscribeExecutionDataFromLatest(ctx, s.criteria)

			wg := &sync.WaitGroup{}
			wg.Add(2)
			go s.writer(wg, cancel, test.backfilledBlocks)
			go s.reader(wg, sub, firstExpectedBlockHeight, context.Canceled)

			wg.Wait()
		})
	}
}

func (s *BackendExecutionDataSuite2) TestGetExecutionData() {
	s.mockDataProviderFuncState()

	backend := s.createExecutionDataBackend()

	actualExecData, metadata, err :=
		backend.GetExecutionDataByBlockID(context.Background(), s.sporkRootBlock.ID(), s.criteria)
	expectedExecData := s.blockIDToExecutionDataMap[s.sporkRootBlock.ID()]

	require.Nil(s.T(), err)
	require.NotEmpty(s.T(), metadata)
	require.Equal(s.T(), expectedExecData, actualExecData)
}

func (s *BackendExecutionDataSuite2) TestExecutionDataProviderFuncErrors() {
	tests := []struct {
		name        string
		expectedErr error
		mockState   func(s *BackendExecutionDataSuite2)
	}{
		{
			name:        "block is not finalized",
			expectedErr: storage.ErrNotFound,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, storage.ErrNotFound
					}).
					Once()
			},
		},
		{
			name:        "block not found",
			expectedErr: optimistic_sync.ErrBlockNotFound,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, optimistic_sync.ErrBlockNotFound
					}).
					Once()
			},
		},
		{
			name:        "fork abandoned",
			expectedErr: optimistic_sync.ErrForkAbandoned,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, optimistic_sync.ErrForkAbandoned
					}).
					Once()
			},
		},
		{
			name:        "unexpected error",
			expectedErr: assert.AnError,
			mockState: func(s *BackendExecutionDataSuite2) {
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, assert.AnError
					}).
					Once()
			},
		},
	}

	s.headers.
		On("BlockIDByHeight", mock.Anything).
		Return(func(height uint64) (flow.Identifier, error) {
			block, ok := s.blocksHeightToBlockMap[height]
			if !ok {
				return flow.ZeroID, storage.ErrNotFound
			}
			return block.ID(), nil
		})

	executionDataReader := osyncmock.NewBlockExecutionDataReader(s.T())
	s.executionDataSnapshot.
		On("BlockExecutionData").
		Return(executionDataReader)

	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(s.executionDataSnapshot, nil)

	executionDataReader.
		On("ByBlockID", mock.Anything, mock.Anything).
		Return(func(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error) {
			ed, ok := s.blockIDToExecutionDataMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}

			return &execution_data.BlockExecutionDataEntity{
				BlockExecutionData: ed,
				ExecutionDataID:    unittest.IdentifierFixture(),
			}, nil
		})

	// stream the first 2 blocks for each subtest with no errors
	s.executionResultProvider.
		On("ExecutionResultInfo", mock.Anything, mock.Anything).
		Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
			return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
		}).
		Times(2 * len(tests))

	s.executionDataTracker.
		On("GetStartHeightFromBlockID", mock.AnythingOfType("flow.Identifier")).
		Return(
			func(ID flow.Identifier) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromBlockID(ID)
			},
			nil,
		).
		Times(len(tests))

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
	)
	firstExpectedBlockHeight := s.sporkRootBlock.Height

	for _, test := range tests {
		s.Run(test.name, func() {
			test.mockState(s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := backend.SubscribeExecutionDataFromStartBlockID(ctx, s.sporkRootBlock.ID(), s.criteria)

			wg := &sync.WaitGroup{}
			wg.Add(2)

			// writer sends 2 blocks
			go func() {
				defer wg.Done()

				for range 3 {
					s.executionDataBroadcaster.Publish()
				}

				cancel()
			}()

			go s.reader(wg, sub, firstExpectedBlockHeight, test.expectedErr)

			wg.Wait()
		})
	}
}

func (s *BackendExecutionDataSuite2) createExecutionDataBackend() *ExecutionDataBackend {
	b := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
	)
	return b
}

// writer notifies that execution data broadcaster that new execution data is available and can be streamed to
// subscription. it also skips notifying for the backfilled blocks.
func (s *BackendExecutionDataSuite2) writer(wg *sync.WaitGroup, cancel context.CancelFunc, backfilledBlocks int) {
	defer wg.Done()
	defer cancel() // cancel streamer/subscription context

	i := 0

	for _, execData := range s.executionDataList {
		s.log.Info().Msgf("publishing execution data %v for block ID %d", execData, execData.BlockID)

		// don't publish anything, emulating the current block has been backfilled before
		if i < backfilledBlocks {
			i += 1
			continue
		}

		// this will trigger the streamer to write the execution data to the subscription
		// as we use height based subscriptions, the execution data are written in the order
		// of block heights.
		s.executionDataBroadcaster.Publish()
	}
}

// reader reads the execution data from the subscription and compares it with the expected execution data
// for the particular block. As the subscription is height based, we expect execution data to come in the
// block height order.
func (s *BackendExecutionDataSuite2) reader(wg *sync.WaitGroup, sub subscription.Subscription, firstHeight uint64, expectedErr error) {
	defer wg.Done()

	currentHeight := firstHeight

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1
	}

	require.ErrorIs(s.T(), sub.Err(), expectedErr)
}

// mockDataProviderFuncState sets up the mocked the data provider function state for test scenarios involving execution data.
// It configures the behavior and expectations of the headers and result provider mocks with concurrency considerations.
//
// As the data provider function is used in the concurrent context, we can't know how many times each call on the mocks
// will be performed, so we use maybe() instead of Times() for every mock in this function.
func (s *BackendExecutionDataSuite2) mockDataProviderFuncState() {
	// we can't know exactly how many times it will be called because it's called by the streamer code
	// where the concurrency plays a role.
	//
	// also, for full backfill tests, it doesn't get called at all, because the
	// streamer's code cancels the context just after sending all the available data to the subscription.
	// due to this, we can't remove Maybe() saying it must be called at least once.
	s.headers.
		On("BlockIDByHeight", mock.Anything).
		Return(func(height uint64) (flow.Identifier, error) {
			block, ok := s.blocksHeightToBlockMap[height]
			if !ok {
				return flow.ZeroID, storage.ErrNotFound
			}
			return block.ID(), nil
		}).
		Maybe()

	s.mockResultProvider()
}

// mockResultProvider configures and sets up mocked behaviors for execution state cache, execution data snapshot,
// execution data reader, and execution result provider, enabling controlled test scenarios.
//
// As these abstractions are used in the concurrent context, we can't know exactly how many times each call on a mock
// is performed, so we use maybe() instead of Times() for every mock in this function.
func (s *BackendExecutionDataSuite2) mockResultProvider() {
	executionDataReader := osyncmock.NewBlockExecutionDataReader(s.T())

	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(s.executionDataSnapshot, nil).
		Maybe()

	s.executionDataSnapshot.
		On("BlockExecutionData").
		Return(executionDataReader).
		Maybe()

	executionDataReader.
		On("ByBlockID", mock.Anything, mock.Anything).
		Return(func(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error) {
			ed, ok := s.blockIDToExecutionDataMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}

			return &execution_data.BlockExecutionDataEntity{
				BlockExecutionData: ed,
				ExecutionDataID:    unittest.IdentifierFixture(),
			}, nil
		}).
		Maybe()

	s.executionResultProvider.
		On("ExecutionResultInfo", mock.Anything, mock.Anything).
		Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
			return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
		}).
		Maybe()
}
