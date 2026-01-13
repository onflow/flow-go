package backend

import (
	"context"
	"errors"
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

type BackendExecutionDataSuite struct {
	suite.Suite

	g *fixtures.GeneratorSuite

	log      zerolog.Logger
	state    *protocolmock.State
	params   *protocolmock.Params
	snapshot *protocolmock.Snapshot
	headers  *storagemock.Headers
	receipts *storagemock.ExecutionReceipts

	// execution data stuff
	executionDataBroadcaster *engine.Broadcaster
	subscriptionFactory      *subscription.SubscriptionHandler
	executionDataTracker     tracker.ExecutionDataTracker

	// optimistic sync stuff
	executionResultProviderMock *osyncmock.ExecutionResultInfoProvider
	executionResultProvider     optimistic_sync.ExecutionResultInfoProvider
	executionDataReader         *osyncmock.BlockExecutionDataReader
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

// TestBackendExecutionDataSuite2 runs the BackendExecutionDataSuite test suite.
func TestBackendExecutionDataSuite2(t *testing.T) {
	suite.Run(t, new(BackendExecutionDataSuite))
}

// SetupTest initializes the test suite with necessary mocks and fixtures.
func (s *BackendExecutionDataSuite) SetupTest() {
	s.log = unittest.Logger()

	s.g = fixtures.NewGeneratorSuite()

	// blocks and execution data for the tests
	s.blocks = s.g.Blocks().List(5)
	s.T().Logf("first block height: %d, last block height: %d", s.blocks[0].Height, s.blocks[len(s.blocks)-1].Height)

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

		execData := s.g.BlockExecutionDatas().Fixture(
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
	s.executionDataTracker = tracker.NewExecutionDataTracker(
		s.log,
		s.state,
		s.sporkRootBlock.Height,
		s.headers,
		s.executionDataBroadcaster,
		s.blocks[len(s.blocks)-1].Height,
	)

	s.subscriptionFactory = subscription.NewSubscriptionHandler(
		s.log,
		s.executionDataBroadcaster,
		subscription.DefaultSendTimeout,
		subscription.DefaultResponseLimit,
		subscription.DefaultSendBufferSize,
	)

	// optimistic sync stuff
	executionNodeSelector := execution_result.NewExecutionNodeSelector(
		s.preferredExecutionNodeIDs,
		s.fixedExecutionNodes.NodeIDs(),
	)

	// these are used in provider constructor
	s.state.On("Params").Return(s.params).Once()
	s.params.On("SporkRootBlock").Return(s.sporkRootBlock, nil).Once()

	resolver := execution_result.NewSealingStatusResolver(s.headers, s.state)
	s.executionResultProvider = execution_result.NewExecutionResultInfoProvider(
		s.log,
		s.state,
		s.receipts,
		s.headers,
		executionNodeSelector,
		s.criteria,
		resolver,
	)

	s.executionResultProviderMock = osyncmock.NewExecutionResultInfoProvider(s.T())
	s.executionDataReader = osyncmock.NewBlockExecutionDataReader(s.T())
	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
	s.criteria = optimistic_sync.DefaultCriteria
}

// TestSubscribeExecutionData verifies that the subscription to execution data works correctly
// starting from the spork root block ID. It ensures that the execution data is received
// sequentially and matches the expected data.
func (s *BackendExecutionDataSuite) TestSubscribeExecutionData() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.sporkRootBlock,
	)
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

	// we only break out of the above loop when the subscription's channel is closed. this happens after
	// the context cancellation is processed. At this point, the subscription should contain the error.
	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromNonRoot verifies that subscribing with a start block
// ID different from the spork root works as expected. We start from the block right
// after the spork root and stream all remaining blocks.
func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromNonRoot() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	// called on the start by tracker
	startBlock := s.blocksHeightToBlockMap[s.sporkRootBlock.Height+1]

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.sporkRootBlock,
	)

	// start from the block right after the spork root
	currentHeight := startBlock.Height

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := backend.SubscribeExecutionData(
		ctx,
		startBlock.ID(),
		0, // either id or height must be provided
		s.criteria,
	)

	received := 0
	expected := len(s.blocks) - 1 // streaming all blocks except the spork root

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

	// we only break out of the above loop when the subscription's channel is closed. this happens after
	// the context cancellation is processed. At this point, the subscription should contain the error.
	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromStartHeight verifies that the subscription can start from a specific
// block height. It ensures that the correct block header is retrieved and data streaming starts
// from the correct block.
func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromStartHeight() {
	s.mockSubscribeFuncState()

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	s.headers.
		On("ByHeight", s.sporkRootBlock.Height).
		Return(s.sporkRootBlock.ToHeader(), nil).
		Once()

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.sporkRootBlock,
	)
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

	// we only break out of the above loop when the subscription's channel is closed. this happens after
	// the context cancellation is processed. At this point, the subscription should contain the error.
	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromStartID verifies that the subscription can start from a specific
// block ID. It checks that the start height is correctly resolved from the block ID and data
// streaming proceeds.
func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromStartID() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.sporkRootBlock,
	)
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

	// we only break out of the above loop when the subscription's channel is closed. this happens after
	// the context cancellation is processed. At this point, the subscription should contain the error.
	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestSubscribeExecutionDataFromLatest verifies that the subscription can start from the latest
// available finalized block. It ensures that the start height is correctly determined and data
// streaming begins.
func (s *BackendExecutionDataSuite) TestSubscribeExecutionDataFromLatest() {
	s.mockSubscribeFuncState()

	s.headers.
		On("ByHeight", s.sporkRootBlock.Height).
		Return(s.sporkRootBlock.ToHeader(), nil).
		Once()

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.blocks[0].ToHeader(), nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.sporkRootBlock,
	)

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

	// we only break out of the above loop when the subscription's channel is closed. this happens after
	// the context cancellation is processed. At this point, the subscription should contain the error.
	require.ErrorIs(s.T(), sub.Err(), context.Canceled)
}

// TestGetExecutionData verifies the retrieval of execution data for a specific block ID.
// It checks that the data and metadata are returned correctly.
func (s *BackendExecutionDataSuite) TestGetExecutionData() {
	s.mockExecutionResultProviderState()
	s.snapshot.
		On("SealedResult").
		Return(s.executionResults[0], nil, nil).
		Once()

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProvider,
		s.executionStateCache,
		s.sporkRootBlock,
	)

	actualExecData, metadata, err :=
		backend.GetExecutionDataByBlockID(context.Background(), s.sporkRootBlock.ID(), s.criteria)
	expectedExecData := s.blockIDToExecutionDataMap[s.sporkRootBlock.ID()]

	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), metadata)
	require.Equal(s.T(), expectedExecData, actualExecData)
}

// TestGetExecutionData_Errors verifies how GetExecutionDataByBlockID handles various error
// conditions, such as missing data, block not found, fork abandoned, and unexpected system errors.
func (s *BackendExecutionDataSuite) TestGetExecutionData_Errors() {
	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProviderMock,
		s.executionStateCache,
		s.sporkRootBlock,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := s.sporkRootBlock

	s.Run("execution result info returns block not found", func() {
		s.executionResultProviderMock.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, optimistic_sync.ErrBlockBeforeNodeHistory).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("execution result info returns fork abandoned", func() {
		s.executionResultProviderMock.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(nil, optimistic_sync.NewExecutionResultNotReadyError("parent mismatch")).
			Once()

		execDataRes, metadata, err :=
			backend.GetExecutionDataByBlockID(ctx, block.ID(), s.criteria)
		assert.Nil(s.T(), execDataRes)
		assert.Nil(s.T(), metadata)
		require.Error(s.T(), err)
		require.True(s.T(), access.IsDataNotFoundError(err))
	})

	s.Run("execution result info returns unexpected error", func() {
		// any unexpected error should be wrapped by access.RequireNoError, i.e., returned as-is
		unexpected := errors.New("unexpected error")
		s.executionResultProviderMock.
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
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Twice()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		// first return a valid exec result info
		s.executionResultProviderMock.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(s.executionResultProvider.ExecutionResultInfo).
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
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Twice()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		s.executionResultProviderMock.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(s.executionResultProvider.ExecutionResultInfo).
			Once()

		unexpected := errors.New("unexpected error")
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
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Twice()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		s.executionResultProviderMock.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(s.executionResultProvider.ExecutionResultInfo).
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
			On("AtBlockID", block.ID()).
			Return(s.snapshot, nil).
			Twice()

		s.snapshot.
			On("Identities", mock.Anything).
			Return(s.fixedExecutionNodes, nil).
			Once()

		s.snapshot.
			On("SealedResult").
			Return(s.executionResults[0], nil, nil).
			Once()

		s.executionResultProviderMock.
			On("ExecutionResultInfo", block.ID(), mock.Anything).
			Return(s.executionResultProvider.ExecutionResultInfo).
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
func (s *BackendExecutionDataSuite) TestExecutionDataProviderErrors() {
	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

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
				s.executionResultProviderMock.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(s.executionResultProvider.ExecutionResultInfo).
					Once()

				s.executionResultProviderMock.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, storage.ErrNotFound
					}).
					Once()
			},
		},
		{
			name:        "block not found",
			expectedErr: optimistic_sync.ErrBlockBeforeNodeHistory,
			mockState: func() {
				// stream the first block normally, then stream an error
				s.executionResultProviderMock.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(s.executionResultProvider.ExecutionResultInfo).
					Once()

				s.executionResultProviderMock.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, optimistic_sync.ErrBlockBeforeNodeHistory
					}).
					Once()
			},
		},
		{
			name:        "unexpected error",
			expectedErr: assert.AnError,
			mockState: func() {
				// stream the first block normally, then stream an error
				s.executionResultProviderMock.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(s.executionResultProvider.ExecutionResultInfo).
					Once()

				s.executionResultProviderMock.
					On("ExecutionResultInfo", mock.Anything, mock.Anything).
					Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
						return nil, assert.AnError
					}).
					Once()
			},
		},
	}

	s.mockSubscribeFuncState()

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProviderMock,
		s.executionStateCache,
		s.sporkRootBlock,
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

// TestExecutionResultNotReadyError verifies that the data provider handles temporary or
// ignorable errors correctly. It ensures that the provider retries or waits when encountering
// errors like missing required executors, rather than terminating the subscription immediately,
// until the context is canceled.
func (s *BackendExecutionDataSuite) TestExecutionResultNotReadyError() {
	s.headers.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			block, ok := s.blocksIDToBlockMap[blockID]
			if !ok {
				return nil, storage.ErrNotFound
			}
			return block.ToHeader(), nil
		})

	s.state.
		On("AtHeight", mock.Anything).
		Return(s.snapshot, nil)

	s.executionResultProviderMock.
		On("ExecutionResultInfo", mock.Anything, mock.Anything).
		Return(func(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
			// after an 'ignorable' error occurs, the streamer goes to sleep waiting for the notification
			// that the new data is available.
			// since we are about to return an error, we notify the streamer in advance. this will cause it
			// to wake up and try again with a new mock.
			s.executionDataBroadcaster.Publish()

			return nil, optimistic_sync.NewExecutionResultNotReadyError("required executors not found")
		}).
		Once()

	// called `len(s.blocks) - 1` times because we return the error for the first block
	s.executionResultProviderMock.
		On("ExecutionResultInfo", mock.Anything, mock.Anything).
		Return(s.executionResultProvider.ExecutionResultInfo).
		Times(len(s.blocks) - 1)

	s.mockSubscribeFuncState()

	backend := NewExecutionDataBackend(
		s.log,
		s.state,
		s.headers,
		s.subscriptionFactory,
		s.executionDataTracker,
		s.executionResultProviderMock,
		s.executionStateCache,
		s.sporkRootBlock,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	currentHeight := s.sporkRootBlock.Height
	sub := backend.SubscribeExecutionDataFromStartBlockID(ctx, s.sporkRootBlock.ID(), s.criteria)

	// cancel after we have received all expected blocks.
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

// mockSubscribeFuncState sets up mock expectations for Subscribe* functions that require access to the
// execution state.
func (s *BackendExecutionDataSuite) mockSubscribeFuncState() {
	s.params.On("SporkRootBlockHeight").Return(s.sporkRootBlock.Height, nil)
	s.params.On("SporkRootBlock").Return(s.sporkRootBlock, nil)
	s.state.On("Params").Return(s.params)

	s.receipts.
		On("ByBlockID", mock.Anything).
		Return(func(blockID flow.Identifier) (flow.ExecutionReceiptList, error) {
			if blockID == s.sporkRootBlock.ID() {
				return nil, errors.New("no execution receipts exist for spork root block")
			}

			return s.blockIDToReceiptsMap[blockID], nil
		})

	s.headers.
		On("BlockIDByHeight", mock.Anything).
		Return(func(height uint64) (flow.Identifier, error) {
			block, ok := s.blocksHeightToBlockMap[height]
			if !ok {
				return flow.ZeroID, storage.ErrNotFound
			}
			return block.ID(), nil
		})

	s.mockExecutionResultProviderState()
}

// mockExecutionResultProviderState sets up mock expectations for the code that calls the execution result provider.
func (s *BackendExecutionDataSuite) mockExecutionResultProviderState() {
	s.snapshot.
		On("Identities", mock.Anything).
		Return(s.fixedExecutionNodes, nil)

	s.state.
		On("AtBlockID", mock.Anything).
		Return(s.snapshot, nil)

	s.executionDataReader.
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

	s.executionDataSnapshot.
		On("BlockExecutionData").
		Return(s.executionDataReader)

	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(s.executionDataSnapshot, nil)
}
