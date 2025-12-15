package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	trackermock "github.com/onflow/flow-go/engine/access/subscription/tracker/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/execution_result"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
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

	executionResults     []*flow.ExecutionResult
	blockIDToReceiptsMap map[flow.Identifier]flow.ExecutionReceiptList

	// execution node configuration
	fixedExecutionNodes       flow.IdentityList
	preferredExecutionNodeIDs flow.IdentifierList
}

// TestBackendExecutionDataSuite2 runs the BackendExecutionDataSuite2 test suite.
func TestBackendExecutionDataSuite2(t *testing.T) {
	suite.Run(t, new(BackendExecutionDataSuite2))
}

// SetupTest initializes the test suite with necessary mocks and fixtures.
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
	s.blockIDToReceiptsMap = make(map[flow.Identifier]flow.ExecutionReceiptList)

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
		// an identical, properly linked execution result.
		receipt2.ExecutionResult = receipt1.ExecutionResult
		prevResultID = receipt1.ExecutionResult.ID()

		receipt1.ExecutorID = s.fixedExecutionNodes[0].NodeID
		receipt2.ExecutorID = s.fixedExecutionNodes[1].NodeID

		// store execution result for this block (shared by both receipts)
		s.executionResults[i] = &receipt1.ExecutionResult

		receipts := flow.ExecutionReceiptList{receipt1, receipt2}
		s.blockIDToReceiptsMap[block.ID()] = receipts
	}

	s.snapshot = protocolmock.NewSnapshot(s.T())
	s.params = protocolmock.NewParams(s.T())
	s.state = protocolmock.NewState(s.T())
	s.headers = storagemock.NewHeaders(s.T())

	// execution data stuff
	s.executionDataBroadcaster = engine.NewBroadcaster()
	s.executionDataTrackerReal = tracker.NewExecutionDataTracker(
		s.log,
		s.state,
		s.sporkRootBlock.Height,
		s.headers,
		s.executionDataBroadcaster,
		s.sporkRootBlock.Height,
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
}

// TestSubscribeExecutionData verifies that the subscription to execution data works correctly
// starting from the spork root block ID. It ensures that the execution data is received
// sequentially and matches the expected data.
func (s *BackendExecutionDataSuite2) TestSubscribeExecutionData() {
	// the spork root block (index 0) is a special case and doesn't require fetching execution data
	// from the provider/storage, hence -1.
	dataCallsCount := len(s.blocks) - 1

	// the tracker is called for each block to check if it's finalized, plus one extra call
	// when the streamer checks the next height which is not yet ready.
	trackerCallsCount := len(s.blocks) + 1

	s.initRealExecutionResultProvider()

	// SporkRootBlockHeight is checked for each block processed by NextData.
	paramHeightCalls := len(s.blocks)
	// SporkRootBlock is checked during initialization (start height check) and for the root block processing.
	paramBlockCalls := 2
	s.mockParams(paramHeightCalls, paramBlockCalls)
	s.mockReceipts(s.blocks)
	s.mockDataProviderState(dataCallsCount, trackerCallsCount)

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

	// cancel after we have received all expected blocks to avoid waiting for
	// streamer/provider to hit the "block not ready" or missing height path.
	received := 0
	expected := len(s.blocks)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1

		received++
		if received == expected {
			// we’ve validated enough; stop the stream.
			cancel()
		}
	}

	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromStartHeight verifies that the subscription can start from a specific
// block height. It ensures that the correct block header is retrieved and data streaming starts
// from the correct block.
func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromStartHeight() {
	// the spork root block (index 0) is a special case and doesn't require fetching execution data
	// from the provider/storage, hence -1.
	dataCallsCount := len(s.blocks) - 1

	// the tracker is called for each block to check if it's finalized, plus one extra call
	// when the streamer checks the next height which is not yet ready.
	trackerCallsCount := len(s.blocks) + 1

	s.initRealExecutionResultProvider()

	// SporkRootBlockHeight is checked for each block processed by NextData.
	paramHeightCalls := len(s.blocks)
	// SporkRootBlock is checked only for the root block processing.
	paramBlockCalls := 1
	s.mockParams(paramHeightCalls, paramBlockCalls)
	s.mockReceipts(s.blocks)

	s.mockDataProviderState(dataCallsCount, trackerCallsCount)

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

	// cancel after we have received all expected blocks to avoid waiting for
	// streamer/provider to hit the "block not ready" or missing height path.
	received := 0
	expected := len(s.blocks)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1

		received++
		if received == expected {
			// we’ve validated enough; stop the stream.
			cancel()
		}
	}

	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromStartID verifies that the subscription can start from a specific
// block ID. It checks that the start height is correctly resolved from the block ID and data
// streaming proceeds.
func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromStartID() {
	// the spork root block (index 0) is a special case and doesn't require fetching execution data
	// from the provider/storage, hence -1.
	dataCallsCount := len(s.blocks) - 1

	// the tracker is called for each block to check if it's finalized, plus one extra call
	// when the streamer checks the next height which is not yet ready.
	trackerCallsCount := len(s.blocks) + 1

	s.initRealExecutionResultProvider()

	// SporkRootBlockHeight is checked for each block processed by NextData.
	paramHeightCalls := len(s.blocks)
	// SporkRootBlock is checked during initialization (start height check) and for the root block processing.
	paramBlockCalls := 2
	s.mockParams(paramHeightCalls, paramBlockCalls)
	s.mockReceipts(s.blocks)

	s.mockDataProviderState(dataCallsCount, trackerCallsCount)

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

	// cancel after we have received all expected blocks to avoid waiting for
	// streamer/provider to hit the "block not ready" or missing height path.
	received := 0
	expected := len(s.blocks)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1

		received++
		if received == expected {
			// we’ve validated enough; stop the stream.
			cancel()
		}
	}

	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromLatest verifies that the subscription can start from the latest
// available finalized block. It ensures that the start height is correctly determined and data
// streaming begins.
func (s *BackendExecutionDataSuite2) TestSubscribeExecutionDataFromLatest() {
	// the spork root block (index 0) is a special case and doesn't require fetching execution data
	// from the provider/storage, hence -1.
	dataCallsCount := len(s.blocks) - 1

	// the tracker is called for each block to check if it's finalized, plus one extra call
	// when the streamer checks the next height which is not yet ready.
	trackerCallsCount := len(s.blocks) + 1

	s.initRealExecutionResultProvider()

	// SporkRootBlockHeight is checked for each block processed by NextData.
	paramHeightCalls := len(s.blocks)
	// SporkRootBlock is checked only for the root block processing.
	paramBlockCalls := 1
	s.mockParams(paramHeightCalls, paramBlockCalls)
	s.mockReceipts(s.blocks)

	s.mockDataProviderState(dataCallsCount, trackerCallsCount)

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

	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.blocks[0].ToHeader(), nil).Once()

	backend := s.createExecutionDataBackend()

	sub := backend.SubscribeExecutionDataFromLatest(ctx, s.criteria)
	currentHeight := s.sporkRootBlock.Height

	// cancel after we have received all expected blocks to avoid waiting for
	// streamer/provider to hit the "block not ready" or missing height path.
	received := 0
	expected := len(s.blocks)

	for value := range sub.Channel() {
		actualExecutionData, ok := value.(*ExecutionDataResponse)
		require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
		require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

		block := s.blocksHeightToBlockMap[currentHeight]
		expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
		require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

		currentHeight += 1

		received++
		if received == expected {
			// we’ve validated enough; stop the stream.
			cancel()
		}
	}

	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestGetExecutionData verifies the retrieval of execution data for a specific block ID.
// It checks that the data and metadata are returned correctly.
func (s *BackendExecutionDataSuite2) TestGetExecutionData() {
	s.initRealExecutionResultProvider()
	s.mockExecutionResultProvider(1)
	s.state.On("AtBlockID", s.sporkRootBlock.ID()).Return(s.snapshot, nil).Once()
	s.snapshot.On("SealedResult").Return(s.executionResults[0], nil, nil).Once()

	backend := s.createExecutionDataBackend()

	actualExecData, metadata, err :=
		backend.GetExecutionDataByBlockID(context.Background(), s.sporkRootBlock.ID(), s.criteria)
	expectedExecData := s.blockIDToExecutionDataMap[s.sporkRootBlock.ID()]

	require.Nil(s.T(), err)
	require.NotEmpty(s.T(), metadata)
	require.Equal(s.T(), expectedExecData, actualExecData)
}

// TestGetExecutionData_Errors verifies how GetExecutionDataByBlockID handles various error
// conditions, such as missing data, block not found, fork abandoned, and unexpected system errors.
func (s *BackendExecutionDataSuite2) TestGetExecutionData_Errors() {
	s.initRealExecutionResultProvider()

	backend := s.createExecutionDataBackend()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := s.sporkRootBlock

	s.Run("execution result info returns data not found", func() {
		// provider returns storage.ErrNotFound which should map to access.DataNotFoundError
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, storage.ErrNotFound).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("execution result info returns block not found", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, optimistic_sync.ErrBlockNotFound).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("execution result info returns fork abandoned", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, optimistic_sync.ErrForkAbandoned).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsPreconditionFailedError(err))
	})

	s.Run("execution result info returns unexpected error", func() {
		// any unexpected error should be wrapped by access.RequireNoError, i.e., returned as-is
		unexpected := assert.AnError
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, unexpected).
			Once()

		exception := fmt.Errorf("failed to get execution result info for block %s: %w", block.ID(), unexpected)
		ctxSignaler := irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, exception)
		ctxIrr := irrecoverable.WithSignalerContext(ctx, ctxSignaler)

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctxIrr, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		assert.Error(s.T(), err)
	})

	s.Run("snapshot returns data not found", func() {
		s.state.Test(s.T())
		s.snapshot.Test(s.T())

		s.state.
			On("Final").
			Return(s.snapshot, nil).
			Once()

		s.state.
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Once()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		// first return a valid exec result info
		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
				info, err := s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
				return info, err
			}).
			Once()

		// Snapshot not found maps to DataNotFoundError
		s.executionStateCache.
			On("Snapshot", mock.Anything).
			Return(nil, storage.ErrNotFound).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("snapshot returns unexpected error", func() {
		s.state.Test(s.T())
		s.snapshot.Test(s.T())

		s.state.
			On("Final").
			Return(s.snapshot, nil).
			Once()

		s.state.
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Once()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
				info, err := s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
				return info, err
			}).
			Once()

		unexpected := assert.AnError
		s.executionStateCache.
			On("Snapshot", mock.Anything).
			Return(nil, unexpected).
			Once()

		// RequireErrorIs will treat this as irrecoverable; expect the same exception
		exception := fmt.Errorf("%v", unexpected)
		ctxSignaler := irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, exception)
		ctxIrr := irrecoverable.WithSignalerContext(ctx, ctxSignaler)

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctxIrr, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		assert.Error(s.T(), err)
	})

	s.Run("missing exec data in reader (not found)", func() {
		s.state.Test(s.T())
		s.snapshot.Test(s.T())

		s.state.
			On("Final").
			Return(s.snapshot, nil).
			Once()

		s.state.
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Once()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
				info, err := s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
				return info, err
			}).
			Once()

		// provide snapshot and reader that returns not found
		reader := osyncmock.NewBlockExecutionDataReader(s.T())
		s.executionStateCache.
			On("Snapshot", mock.Anything).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("BlockExecutionData").
			Return(reader).
			Once()
		reader.
			On("ByBlockID", mock.Anything, block.ID()).
			Return(nil, storage.ErrNotFound).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("reader returns unexpected error", func() {
		s.state.Test(s.T())
		s.snapshot.Test(s.T())

		s.state.
			On("Final").
			Return(s.snapshot, nil).
			Once()

		s.state.
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Once()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		s.executionResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
				info, err := s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
				return info, err
			}).
			Once()

		reader := osyncmock.NewBlockExecutionDataReader(s.T())
		s.executionStateCache.
			On("Snapshot", mock.Anything).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("BlockExecutionData").
			Return(reader).
			Once()
		reader.
			On("ByBlockID", mock.Anything, block.ID()).
			Return(nil, storage.ErrDataMismatch).
			Once()

		exception := fmt.Errorf("unexpected error getting execution data: %w", storage.ErrDataMismatch)
		ctxSignaler := irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, exception)
		ctxIrr := irrecoverable.WithSignalerContext(ctx, ctxSignaler)

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctxIrr, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		assert.Error(s.T(), err)
	})
}

// TestExecutionDataProviderErrors tests the error handling capabilities of the execution data
// provider used by the subscription. It simulates various error scenarios from the execution
// result provider and storage to ensure the subscription terminates with the expected error.
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
				s.mockReceipts([]*flow.Block{s.blocks[1]})
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
				s.mockReceipts([]*flow.Block{s.blocks[1]})
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
				s.mockReceipts([]*flow.Block{s.blocks[1]})
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
				s.mockReceipts([]*flow.Block{s.blocks[1]})
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
	}

	s.initRealExecutionResultProvider()

	// SporkRootBlockHeight is called 3 times per test:
	// 1. NextData(root block)
	// 2. NextData(block 1)
	// 3. NextData(block 2) - where it fails
	paramHeightCalls := 3 * len(tests)

	// SporkRootBlock is called 2 times per test:
	// 1. GetStartHeightFromBlockID (initial check)
	// 2. NextData(root block)
	paramBlockCalls := 2 * len(tests)

	s.mockParams(paramHeightCalls, paramBlockCalls)

	s.state.
		On("Final").
		Return(s.snapshot, nil).
		Times(len(tests))

	s.snapshot.
		On("Identities", mock.Anything).
		Return(s.fixedExecutionNodes, nil).
		Times(len(tests))

	s.headers.
		On("BlockIDByHeight", mock.Anything).
		Return(func(height uint64) (flow.Identifier, error) {
			block, ok := s.blocksHeightToBlockMap[height]
			if !ok {
				return flow.ZeroID, storage.ErrNotFound
			}
			return block.ID(), nil
		})

	s.executionDataTracker.
		On("GetHighestAvailableFinalizedHeight").
		Return(s.blocks[len(s.blocks)-1].Height)

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

// TestExecutionDataProviderIgnorableErrors verifies that the data provider handles temporary or
// ignorable errors correctly. It ensures that the provider retries or waits when encountering
// errors like missing required executors, rather than terminating the subscription immediately,
// until the context is canceled.
func (s *BackendExecutionDataSuite2) TestExecutionDataProviderIgnorableErrors() {
	s.initRealExecutionResultProvider()

	mockStateFunc := func(errToReturn error) {
		// SporkRootBlockHeight is called:
		// - once for each block in s.blocks (len(s.blocks))
		// - plus one extra time because of the retry on the ignorable error
		paramHeightCalls := len(s.blocks) + 1

		// SporkRootBlock is called:
		// - once during initialization (GetStartHeightFromBlockID)
		// - once for the root block processing in NextData
		paramBlockCalls := 2
		s.mockParams(paramHeightCalls, paramBlockCalls)
		s.mockReceipts(s.blocks)

		s.state.
			On("Final").
			Return(s.snapshot, nil).
			Times(len(s.blocks) - 1)

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Times(len(s.blocks) - 1)

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
	}

	tests := []struct {
		name        string
		expectedErr error
		mockState   func()
	}{
		{
			name:        "required executors not found",
			expectedErr: context.Canceled,
			mockState: func() {
				mockStateFunc(optimistic_sync.ErrRequiredExecutorNotFound)
			},
		},
		{
			name:        "not enough agreeing executors",
			expectedErr: context.Canceled,
			mockState: func() {
				mockStateFunc(optimistic_sync.ErrNotEnoughAgreeingExecutors)
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

	s.executionDataTracker.
		On("GetHighestAvailableFinalizedHeight").
		Return(s.blocks[len(s.blocks)-1].Height)

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

			// cancel after we have received all expected blocks to avoid waiting for
			// streamer/provider to hit the "block not ready" or missing height path.
			received := 0
			expected := len(s.blocks)

			for value := range sub.Channel() {
				actualExecutionData, ok := value.(*ExecutionDataResponse)
				require.True(s.T(), ok, "expected *ExecutionDataResponse on the channel")
				require.NotNil(s.T(), actualExecutionData.ExecutionData, "expected non-nil execution data")

				block := s.blocksHeightToBlockMap[currentHeight]
				expectedExecutionData := s.blockIDToExecutionDataMap[block.ID()]
				require.Equal(s.T(), expectedExecutionData, actualExecutionData.ExecutionData)

				currentHeight += 1

				received++
				if received == expected {
					// we’ve validated enough; stop the stream.
					cancel()
				}
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
// It configures the behavior and expectations of the headers and result provider mocks/
func (s *BackendExecutionDataSuite2) mockDataProviderState(dataCallsCount int, trackerCallsCount int) {
	s.headers.
		On("BlockIDByHeight", mock.Anything).
		Return(func(height uint64) (flow.Identifier, error) {
			block, ok := s.blocksHeightToBlockMap[height]
			if !ok {
				return flow.ZeroID, storage.ErrNotFound
			}
			return block.ID(), nil
		}).
		Times(dataCallsCount)

	s.executionDataTracker.
		On("GetHighestAvailableFinalizedHeight").
		Return(s.blocks[len(s.blocks)-1].Height).
		Times(trackerCallsCount)

	s.mockExecutionResultProvider(dataCallsCount)
}

// mockExecutionResultProvider sets up mocked behaviors for execution state cache, execution data snapshot,
// execution data reader, and execution result provider.
func (s *BackendExecutionDataSuite2) mockExecutionResultProvider(expectationsCount int) {
	executionDataReader := osyncmock.NewBlockExecutionDataReader(s.T())

	s.state.
		On("Final").
		Return(s.snapshot, nil).
		Times(expectationsCount)

	s.snapshot.
		On("Identities", mock.Anything).
		Return(s.fixedExecutionNodes, nil).
		Times(expectationsCount)

	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(s.executionDataSnapshot, nil).
		Times(expectationsCount)

	s.executionDataSnapshot.
		On("BlockExecutionData").
		Return(executionDataReader).
		Times(expectationsCount)

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
		Times(expectationsCount)

	s.executionResultProvider.
		On("ExecutionResultInfo", mock.Anything, mock.Anything).
		Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
			return s.executionResultProviderReal.ExecutionResultInfo(blockID, criteria)
		}).
		Times(expectationsCount)
}

func (s *BackendExecutionDataSuite2) initRealExecutionResultProvider() {
	executionNodeSelector := execution_result.NewExecutionNodeSelector(
		s.preferredExecutionNodeIDs,
		s.fixedExecutionNodes.NodeIDs(),
	)

	s.state.On("Params").Return(s.params).Once()
	s.params.On("SporkRootBlock").Return(s.sporkRootBlock, nil).Once()

	s.executionResultProviderReal = execution_result.NewExecutionResultInfoProvider(
		s.log,
		s.state,
		s.receipts,
		executionNodeSelector,
		s.criteria,
	)
}

func (s *BackendExecutionDataSuite2) mockParams(sporkRootBlockHeightCalls int, sporkRootBlockCalls int) {
	totalParamsCalls := sporkRootBlockHeightCalls + sporkRootBlockCalls
	if totalParamsCalls > 0 {
		s.state.
			On("Params").
			Return(s.params).
			Times(totalParamsCalls)
	}

	if sporkRootBlockHeightCalls > 0 {
		s.params.
			On("SporkRootBlockHeight").
			Return(s.sporkRootBlock.Height, nil).
			Times(sporkRootBlockHeightCalls)
	}

	if sporkRootBlockCalls > 0 {
		s.params.
			On("SporkRootBlock").
			Return(s.sporkRootBlock, nil).
			Times(sporkRootBlockCalls)
	}
}

func (s *BackendExecutionDataSuite2) mockReceipts(blocks []*flow.Block) {
	for _, block := range blocks {
		if block.ID() == s.sporkRootBlock.ID() {
			continue
		}

		s.receipts.
			On("ByBlockID", block.ID()).
			Return(s.blockIDToReceiptsMap[block.ID()], nil).
			Once()
	}
}
