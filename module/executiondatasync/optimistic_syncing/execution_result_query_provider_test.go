package pipeline

import (
	"testing"

	"github.com/onflow/flow-go/engine/access/rpc/backend"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// ExecutionResultQueryProviderSuite is a test suite for testing the ExecutionResultQueryProviderImpl.
type ExecutionResultQueryProviderSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	params   *protocol.Params
	log      zerolog.Logger

	receipts *storagemock.ExecutionReceipts

	rootBlock       *flow.Header
	rootBlockResult *flow.ExecutionResult
	provider        *ExecutionResultQueryProviderImpl
}

func TestExecutionResultQueryProvider(t *testing.T) {
	suite.Run(t, new(ExecutionResultQueryProviderSuite))
}

// SetupTest initializes the test suite with mock state and receipts storage.
func (suite *ExecutionResultQueryProviderSuite) SetupTest() {
	t := suite.T()
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = protocol.NewState(t)
	suite.snapshot = protocol.NewSnapshot(t)
	suite.params = protocol.NewParams(t)
	suite.receipts = storagemock.NewExecutionReceipts(t)

	suite.rootBlock = unittest.BlockHeaderFixture()
	suite.rootBlockResult = unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(suite.rootBlock.ID()))
	// This will be used just for the root block
	suite.snapshot.On("SealedResult").Return(suite.rootBlockResult, nil, nil).Maybe()
	suite.state.On("SealedResult", suite.rootBlock.ID()).Return(flow.ExecutionReceiptList{}).Maybe()
	suite.params.On("FinalizedRoot").Return(suite.rootBlock, nil)
	suite.state.On("Params").Return(suite.params)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("AtBlockID", mock.Anything).Return(suite.snapshot).Maybe()

	var err error
	// Initialize the provider with empty preferred and required EN lists for basic tests
	suite.provider, err = NewExecutionResultQueryProviderImpl(
		suite.log,
		suite.state,
		suite.receipts,
		flow.IdentifierList{},
		flow.IdentifierList{},
		Criteria{},
	)
	suite.Require().NoError(err)
}

// setupIdentitiesMock sets up the mock for identity-related calls.
func (suite *ExecutionResultQueryProviderSuite) setupIdentitiesMock(allExecutionNodes flow.IdentityList) {
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return allExecutionNodes.Filter(filter)
		},
		func(flow.IdentityFilter[flow.Identity]) error { return nil })
}

// TestExecutionResultQuery tests the main ExecutionResultQuery function with various scenarios.
func (suite *ExecutionResultQueryProviderSuite) TestExecutionResultQuery() {
	totalReceipts := 5
	block := unittest.BlockFixture()

	// generate execution node identities for each receipt
	allExecutionNodes := unittest.IdentityListFixture(totalReceipts, unittest.WithRole(flow.RoleExecution))

	// create two different execution results to test agreement logic
	executionResult := unittest.ExecutionResultFixture()

	suite.Run("query with required executors", func() {
		receipts := make(flow.ExecutionReceiptList, totalReceipts)
		for i := 0; i < totalReceipts; i++ {
			r := unittest.ReceiptForBlockFixture(&block)
			r.ExecutorID = allExecutionNodes[i].NodeID
			r.ExecutionResult = *executionResult
			receipts[i] = r
		}

		suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
		suite.setupIdentitiesMock(allExecutionNodes)

		// Require specific executors (first two nodes)
		requiredExecutors := allExecutionNodes[0:2].NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 2,
			RequiredExecutors: requiredExecutors,
		}

		query, err := suite.provider.ExecutionResultQuery(block.ID(), criteria)
		suite.Require().NoError(err)

		suite.Assert().ElementsMatch(requiredExecutors, query.ExecutionNodes.NodeIDs())
	})

	suite.Run("query with specific result in fork", func() {
		receipts := make(flow.ExecutionReceiptList, totalReceipts)
		for i := 0; i < totalReceipts; i++ {
			r := unittest.ReceiptForBlockFixture(&block)
			r.ExecutorID = allExecutionNodes[i].NodeID
			r.ExecutionResult = *executionResult
			receipts[i] = r
		}

		suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
		suite.setupIdentitiesMock(allExecutionNodes)

		requiredExecutors := allExecutionNodes[0:2].NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 1,
			RequiredExecutors: requiredExecutors,
			ResultInFork:      executionResult.ID(),
		}

		query, err := suite.provider.ExecutionResultQuery(block.ID(), criteria)
		suite.Require().NoError(err)

		suite.Assert().Equal(criteria.ResultInFork, query.ExecutionResult.ID())
	})

	suite.Run("successful query with different block results", func() {
		otherResult := unittest.ExecutionResultFixture()
		// Create 3 receipts with the same result (executionResult) and 2 with a different result (otherResult)
		receipts := make(flow.ExecutionReceiptList, totalReceipts)
		for i := 0; i < 3; i++ {
			r := unittest.ReceiptForBlockFixture(&block)
			r.ExecutorID = allExecutionNodes[i].NodeID
			r.ExecutionResult = *executionResult
			receipts[i] = r
		}
		for i := 3; i < totalReceipts; i++ {
			r := unittest.ReceiptForBlockFixture(&block)
			r.ExecutorID = allExecutionNodes[i].NodeID
			r.ExecutionResult = *otherResult
			receipts[i] = r
		}

		suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
		suite.setupIdentitiesMock(allExecutionNodes)

		requiredExecutors := allExecutionNodes[0:3].NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 2,
			RequiredExecutors: requiredExecutors,
		}

		query, err := suite.provider.ExecutionResultQuery(block.ID(), criteria)
		suite.Require().NoError(err)

		suite.Require().Equal(executionResult.ID(), query.ExecutionResult.ID())
		suite.Assert().ElementsMatch(requiredExecutors, query.ExecutionNodes.NodeIDs())
	})

	suite.Run("insufficient agreeing executors returns error", func() {
		// Create a fresh block for this test to ensure proper isolation
		insufficientBlock := unittest.BlockFixture()

		// Create a scenario where we have receipts but no result has enough agreeing executors

		// Create only 1 receipt with 1 execution result
		r := unittest.ReceiptForBlockFixture(&insufficientBlock)
		r.ExecutorID = allExecutionNodes[0].NodeID
		r.ExecutionResult = *unittest.ExecutionResultFixture()
		receipts := flow.ExecutionReceiptList{
			r,
		}

		// Set up a separate mock call for this specific block
		suite.receipts.On("ByBlockID", insufficientBlock.ID()).Return(receipts, nil).Once()

		requiredExecutors := allExecutionNodes[0:1].NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 2,
			RequiredExecutors: requiredExecutors,
		}

		_, err := suite.provider.ExecutionResultQuery(insufficientBlock.ID(), criteria)
		suite.Require().Error(err)

		suite.Assert().True(backend.IsInsufficientExecutionReceipts(err))
	})

	suite.Run("required executors not found returns error", func() {
		receipts := make(flow.ExecutionReceiptList, totalReceipts)
		for i := 0; i < totalReceipts; i++ {
			r := unittest.ReceiptForBlockFixture(&block)
			r.ExecutorID = allExecutionNodes[i].NodeID
			r.ExecutionResult = *executionResult
			receipts[i] = r
		}

		suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
		suite.setupIdentitiesMock(allExecutionNodes)

		// Require executors that didn't produce any receipts
		nonExistentExecutors := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution)).NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 2,
			RequiredExecutors: nonExistentExecutors,
		}

		_, err := suite.provider.ExecutionResultQuery(block.ID(), criteria)
		suite.Require().Error(err)

		suite.Assert().True(backend.IsInsufficientExecutionReceipts(err))
	})
}

// TestRootBlockHandling tests the special case handling for root blocks.
func (suite *ExecutionResultQueryProviderSuite) TestRootBlockHandling() {
	allExecutionNodes := unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleExecution))
	suite.setupIdentitiesMock(allExecutionNodes)

	suite.Run("root block returns execution nodes without execution result", func() {
		query, err := suite.provider.ExecutionResultQuery(suite.rootBlock.ID(), Criteria{})
		suite.Require().NoError(err)

		suite.Assert().Equal(suite.rootBlockResult, query.ExecutionResult)
		suite.Assert().Len(query.ExecutionNodes.NodeIDs(), maxNodesCnt)
		suite.Assert().Subset(allExecutionNodes.NodeIDs(), query.ExecutionNodes.NodeIDs())
	})

	suite.Run("root block with required executors", func() {
		requiredExecutors := allExecutionNodes[0:2].NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 1,
			RequiredExecutors: requiredExecutors,
		}

		query, err := suite.provider.ExecutionResultQuery(suite.rootBlock.ID(), criteria)
		suite.Require().NoError(err)

		suite.Assert().Equal(suite.rootBlockResult, query.ExecutionResult)
		suite.Assert().ElementsMatch(query.ExecutionNodes.NodeIDs(), requiredExecutors)
	})
}

// TestPreferredAndRequiredExecutionNodes tests the interaction with preferred and required execution nodes.
func (suite *ExecutionResultQueryProviderSuite) TestPreferredAndRequiredExecutionNodes() {
	block := unittest.BlockFixture()
	allExecutionNodes := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleExecution))
	executionResult := unittest.ExecutionResultFixture()

	// Create receipts from the first 4 execution nodes
	receipts := make(flow.ExecutionReceiptList, 4)
	for i := 0; i < 4; i++ {
		r := unittest.ReceiptForBlockFixture(&block)
		r.ExecutorID = allExecutionNodes[i].NodeID
		r.ExecutionResult = *executionResult
		receipts[i] = r
	}

	suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
	suite.setupIdentitiesMock(allExecutionNodes)

	suite.Run("with preferred execution nodes", func() {
		// Set nodes 1,2,3 as preferred (nodes that have receipts)
		suite.provider.preferredENIdentifiers = allExecutionNodes[1:4].NodeIDs()

		// Criteria are empty to use operator defaults
		query, err := suite.provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		suite.Assert().Subset(suite.provider.preferredENIdentifiers, query.ExecutionNodes.NodeIDs())
	})

	suite.Run("with required execution nodes", func() {
		// clear previous test result
		suite.provider.preferredENIdentifiers = flow.IdentifierList{}
		// Set nodes 2,3,4,5 as required (some have receipts, some don't)
		suite.provider.requiredENIdentifiers = allExecutionNodes[2:6].NodeIDs()

		// Criteria are empty to use operator defaults
		query, err := suite.provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		suite.Assert().Subset(suite.provider.requiredENIdentifiers, query.ExecutionNodes.NodeIDs())
	})
}
