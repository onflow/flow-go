package pipeline

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
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
	headers  *storagemock.Headers

	rootBlock       *flow.Header
	rootBlockResult *flow.ExecutionResult
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
	suite.headers = storagemock.NewHeaders(t)

	suite.rootBlock = unittest.BlockHeaderFixture()
	rootBlockID := suite.rootBlock.ID()
	suite.rootBlockResult = unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(rootBlockID))
	// This will be used just for the root block
	suite.snapshot.On("SealedResult").Return(suite.rootBlockResult, nil, nil).Maybe()
	suite.state.On("SealedResult", rootBlockID).Return(flow.ExecutionReceiptList{}).Maybe()
	suite.params.On("SporkRootBlockHeight").Return(suite.rootBlock.Height, nil)
	suite.headers.On("BlockIDByHeight", suite.rootBlock.Height).Return(rootBlockID, nil)
	suite.state.On("Params").Return(suite.params)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("AtBlockID", mock.Anything).Return(suite.snapshot).Maybe()
}

func (suite *ExecutionResultQueryProviderSuite) createProvider(preferredExecutors flow.IdentifierList, operatorCriteria Criteria) *ExecutionResultQueryProviderImpl {
	provider, err := NewExecutionResultQueryProviderImpl(
		suite.log,
		suite.state,
		suite.headers,
		suite.receipts,
		preferredExecutors,
		operatorCriteria.RequiredExecutors,
		operatorCriteria,
	)
	suite.Require().NoError(err)

	return provider
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

	suite.Run("query with client required executors", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{})

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

		query, err := provider.ExecutionResultQuery(block.ID(), Criteria{
			AgreeingExecutors: 2,
			RequiredExecutors: requiredExecutors,
		})
		suite.Require().NoError(err)

		suite.Assert().ElementsMatch(requiredExecutors, query.ExecutionNodes.NodeIDs())
	})

	suite.Run("successful query with different block results", func() {
		requiredExecutors := allExecutionNodes[0:3].NodeIDs()

		provider := suite.createProvider(flow.IdentifierList{}, Criteria{
			RequiredExecutors: requiredExecutors,
		})

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

		query, err := provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		suite.Require().Equal(executionResult.ID(), query.ExecutionResult.ID())
		suite.Assert().ElementsMatch(requiredExecutors, query.ExecutionNodes.NodeIDs())
	})

	suite.Run("insufficient agreeing executors returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{})

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
		suite.setupIdentitiesMock(allExecutionNodes)

		_, err := provider.ExecutionResultQuery(insufficientBlock.ID(), Criteria{
			AgreeingExecutors: 2,
			RequiredExecutors: allExecutionNodes[0:1].NodeIDs(),
		})
		suite.Require().Error(err)

		suite.Assert().True(common.IsInsufficientExecutionReceipts(err))
	})

	suite.Run("required executors not found returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{})
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
		_, err := provider.ExecutionResultQuery(block.ID(), Criteria{
			RequiredExecutors: unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution)).NodeIDs(),
		})
		suite.Require().Error(err)

		suite.Assert().True(common.IsInsufficientExecutionReceipts(err))
	})
}

// TestRootBlockHandling tests the special case handling for root blocks.
func (suite *ExecutionResultQueryProviderSuite) TestRootBlockHandling() {
	allExecutionNodes := unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleExecution))
	suite.setupIdentitiesMock(allExecutionNodes)

	suite.Run("root block returns execution nodes without execution result", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{})

		query, err := provider.ExecutionResultQuery(suite.rootBlock.ID(), Criteria{})
		suite.Require().NoError(err)

		suite.Assert().Equal(suite.rootBlockResult, query.ExecutionResult)
		suite.Assert().Len(query.ExecutionNodes.NodeIDs(), maxNodesCnt)
		suite.Assert().Subset(allExecutionNodes.NodeIDs(), query.ExecutionNodes.NodeIDs())
	})

	suite.Run("root block with required executors", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{})

		requiredExecutors := allExecutionNodes[0:2].NodeIDs()
		criteria := Criteria{
			AgreeingExecutors: 1,
			RequiredExecutors: requiredExecutors,
		}

		query, err := provider.ExecutionResultQuery(suite.rootBlock.ID(), criteria)
		suite.Require().NoError(err)

		suite.Assert().Equal(suite.rootBlockResult, query.ExecutionResult)
		suite.Assert().ElementsMatch(query.ExecutionNodes.NodeIDs(), requiredExecutors)
	})
}

// TestPreferredAndRequiredExecutionNodes tests the interaction with preferred and required execution nodes.
func (suite *ExecutionResultQueryProviderSuite) TestPreferredAndRequiredExecutionNodes() {
	block := unittest.BlockFixture()
	allExecutionNodes := unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleExecution))
	executionResult := unittest.ExecutionResultFixture()

	numReceipts := 6
	// Create receipts from the first `numReceipts` execution nodes
	receipts := make(flow.ExecutionReceiptList, numReceipts)
	for i := 0; i < numReceipts; i++ {
		r := unittest.ReceiptForBlockFixture(&block)
		r.ExecutorID = allExecutionNodes[i].NodeID
		r.ExecutionResult = *executionResult
		receipts[i] = r
	}

	suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
	suite.setupIdentitiesMock(allExecutionNodes)

	suite.Run("with default criteria", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{})

		// Criteria are empty to use operator defaults
		query, err := provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		expectedExecutors := allExecutionNodes[0:3].NodeIDs()
		actualExecutors := query.ExecutionNodes.NodeIDs()

		suite.Assert().Len(actualExecutors, maxNodesCnt)
		suite.Assert().ElementsMatch(expectedExecutors, actualExecutors)
	})

	suite.Run("with operator preferred executors", func() {
		provider := suite.createProvider(allExecutionNodes[1:5].NodeIDs(), Criteria{})

		// Criteria are empty to use operator defaults
		query, err := provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		actualExecutors := query.ExecutionNodes.NodeIDs()

		suite.Assert().ElementsMatch(provider.preferredENIdentifiers, actualExecutors)
	})

	suite.Run("with operator required executors", func() {
		provider := suite.createProvider(flow.IdentifierList{}, Criteria{
			RequiredExecutors: allExecutionNodes[5:8].NodeIDs(),
		})

		// Criteria are empty to use operator defaults
		query, err := provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		actualExecutors := query.ExecutionNodes.NodeIDs()

		// Just one required executor contains the result
		expectedExecutors := provider.requiredENIdentifiers[0:1]

		suite.Assert().ElementsMatch(expectedExecutors, actualExecutors)
	})

	suite.Run("with both: operator preferred & required executors", func() {
		provider := suite.createProvider(allExecutionNodes[0:1].NodeIDs(), Criteria{
			RequiredExecutors: allExecutionNodes[3:6].NodeIDs(),
		})

		// Criteria are empty to use operator defaults
		query, err := provider.ExecutionResultQuery(block.ID(), Criteria{})
		suite.Require().NoError(err)

		// `preferredENIdentifiers` contain 1 executor, that is not enough, so the logic will get 2 executors from `requiredENIdentifiers` to fill `maxNodesCnt` executors.
		expectedExecutors := append(provider.preferredENIdentifiers, provider.requiredENIdentifiers[0:2]...)
		actualExecutors := query.ExecutionNodes.NodeIDs()

		suite.Assert().Len(actualExecutors, maxNodesCnt)
		suite.Assert().ElementsMatch(expectedExecutors, actualExecutors)
	})

	suite.Run("with client preferred executors", func() {
		provider := suite.createProvider(allExecutionNodes[0:1].NodeIDs(), Criteria{
			RequiredExecutors: allExecutionNodes[2:4].NodeIDs(),
		})

		userCriteria := Criteria{
			RequiredExecutors: allExecutionNodes[5:6].NodeIDs(),
		}

		query, err := provider.ExecutionResultQuery(block.ID(), userCriteria)
		suite.Require().NoError(err)

		suite.Assert().ElementsMatch(userCriteria.RequiredExecutors, query.ExecutionNodes.NodeIDs())
	})
}
