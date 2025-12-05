package backend

import (
	"context"
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
	s.log = unittest.Logger()

	s.fixtureGenerator = fixtures.NewGeneratorSuite(
		fixtures.WithChainID(flow.Testnet),
		fixtures.WithSeed(42),
	)

	// blocks and execution data for the tests
	s.blocks = s.fixtureGenerator.Blocks().List(5)
	s.log.Info().Msgf("first block height: %d, last block height: %d", s.blocks[0].Height, s.blocks[len(s.blocks)-1].Height)

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

		// link this block's result to the previous block's result to form a fork (chain)
		if i > 0 {
			receipt1.ExecutionResult.PreviousResultID = prevResultID
		}

		// make both executors agree on the same execution result for this block,
		// including the correct PreviousResultID set above. We must perform this
		// assignment AFTER updating PreviousResultID to ensure both receipts carry
		// an identical, properly linked execution result. Otherwise, depending on
		// receipt order, optimistic sync may consider the fork abandoned.
		receipt2.ExecutionResult = receipt1.ExecutionResult
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
	s.mockDataProviderState()

	s.executionDataTracker.
		On("GetStartHeight", mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("uint64")).
		Return(
			func(ctx context.Context, ID flow.Identifier, height uint64) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeight(ctx, ID, height)
			},
			nil,
		).
		Once()

	backend := s.createExecutionDataBackend()
	currentHeight := s.sporkRootBlock.Height

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := backend.SubscribeExecutionData(
		ctx,
		s.sporkRootBlock.ID(),
		0, // either id or height must be provided
		s.criteria,
	)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1
	}

	require.ErrorIs(s.T(), sub.Err(), storage.ErrNotFound)
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromStartHeight() {
	s.mockDataProviderState()

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

	backend := s.createExecutionDataBackend()
	currentHeight := s.sporkRootBlock.Height

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := backend.SubscribeExecutionDataFromStartBlockHeight(
		ctx,
		s.sporkRootBlock.Height,
		s.criteria,
	)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1
	}

	require.ErrorIs(s.T(), sub.Err(), storage.ErrNotFound)
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromStartID() {
	s.mockDataProviderState()

	s.executionDataTracker.
		On("GetStartHeightFromBlockID", mock.AnythingOfType("flow.Identifier")).
		Return(
			func(ID flow.Identifier) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromBlockID(ID)
			},
			nil,
		).
		Once()

	backend := s.createExecutionDataBackend()
	currentHeight := s.sporkRootBlock.Height

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := backend.SubscribeExecutionDataFromStartBlockID(
		ctx,
		s.sporkRootBlock.ID(),
		s.criteria,
	)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1
	}

	require.ErrorIs(s.T(), sub.Err(), storage.ErrNotFound)
}

func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromLatest() {
	s.mockDataProviderState()

	s.executionDataTracker.
		On("GetStartHeightFromLatest", mock.Anything).
		Return(
			func(ctx context.Context) (uint64, error) {
				return s.executionDataTrackerReal.GetStartHeightFromLatest(ctx)
			},
			nil,
		).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := s.createExecutionDataBackend()

	sub := backend.SubscribeExecutionDataFromLatest(ctx, s.criteria)
	currentHeight := s.sporkRootBlock.Height

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1
	}

	require.ErrorIs(s.T(), sub.Err(), storage.ErrNotFound)
}

func (s *BackendExecutionDataSuite2) TestGetExecutionData() {
	s.mockDataProviderState()

	backend := s.createExecutionDataBackend()

	actualExecData, metadata, err :=
		backend.GetExecutionDataByBlockID(context.Background(), s.sporkRootBlock.ID(), s.criteria)
	expectedExecData := s.blockIDToExecutionDataMap[s.sporkRootBlock.ID()]

	require.Nil(s.T(), err)
	require.NotEmpty(s.T(), metadata)
	require.Equal(s.T(), expectedExecData, actualExecData)
}

func (s *BackendExecutionDataSuite2) TestExecutionDataProviderErrors() {
	tests := []struct {
		name        string
		expectedErr error
		mockState   func()
	}{
		{
			name:        "block is not finalized",
			expectedErr: storage.ErrNotFound,
			mockState: func() {
				// stream the first block normally, then stream an error
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
					}).
					Once()

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
			mockState: func() {
				// stream the first block normally, then stream an error
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
					}).
					Once()

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
			mockState: func() {
				// stream the first block normally, then stream an error
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
					}).
					Once()

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
			mockState: func() {
				// stream the first block normally, then stream an error
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
					}).
					Once()

				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, assert.AnError
					}).
					Once()
			},
		},
		{
			name:        "required executors not found",
			expectedErr: storage.ErrNotFound,
			mockState: func() {
				// if this error is seen, we continue trying to fetch results until we get a result or other error.
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						// after an 'ignorable' error occures, the streamer goes to sleep waiting for the notification
						// that the new data is available.
						// since we are about to return an error, we notify the streamer in advance. this will cause it
						// to wake up and try again with a new mock.
						s.executionDataBroadcaster.Publish()

						return nil, optimistic_sync.ErrRequiredExecutorNotFound
					}).
					Once()

				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
					}).
					Times(len(s.blocks) - 1) // we expect storage.ErrNotFound to be returned for the unknown height
				// Times is equal to `len(s.blocks)-1)` because the function is never called for the spork root block,
				// as well as for the last block that is not found in storage.
			},
		},
		{
			name:        "not enough agreeing executors",
			expectedErr: storage.ErrNotFound,
			mockState: func() {
				// if this error is seen, we continue trying to fetch results until we get a result or other error.
				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						// after an 'ignorable' error occures, the streamer goes to sleep waiting for the notification
						// that the new data is available.
						// since we are about to return an error, we notify the streamer in advance. this will cause it
						// to wake up and try again with a new mock.
						s.executionDataBroadcaster.Publish()

						return nil, optimistic_sync.ErrNotEnoughAgreeingExecutors
					}).
					Once()

				s.executionResultProvider.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
					}).
					Times(len(s.blocks) - 1) // we expect storage.ErrNotFound to be returned for the unknown height
				// Times is equal to `len(s.blocks)-1)` because the function is never called for the spork root block,
				// as well as for the last block that is not found in storage.
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

	for _, test := range tests {
		s.Run(test.name, func() {
			test.mockState()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			currentHeight := s.sporkRootBlock.Height
			sub := backend.SubscribeExecutionDataFromStartBlockID(ctx, s.sporkRootBlock.ID(), s.criteria)

			for value := range sub.Channel() {
				actualExecutionData, ok := value.(*ExecutionDataResponse)
				require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
				require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

				block := s.blocksHeightToBlockMap[currentHeight]
				expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
				require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

				currentHeight += 1
			}

			require.ErrorIs(s.T(), sub.Err(), test.expectedErr)
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

// mockDataProviderState sets up the mocked state for the dataProvider interface implementation.
// It configures the behavior and expectations of the headers and result provider mocks with concurrency considerations.
//
// As the data provider struct is used in the concurrent context, we can't know how many times each call on the mocks
// will be performed, so we use Maybe() instead of Times() for every mock in this function.
func (s *BackendExecutionDataSuite2) mockDataProviderState() {
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

	s.mockExecutionResultProvider()
}

// mockExecutionResultProvider sets up mocked behaviors for execution state cache, execution data snapshot,
// execution data reader, and execution result provider.
//
// As these abstractions are used in the concurrent context, we can't know exactly how many times each call on a mock
// is performed, so we use Maybe() instead of Times() for every mock in this function.
func (s *BackendExecutionDataSuite2) mockExecutionResultProvider() {
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
