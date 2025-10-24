package execution_result

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// ExecutionResultInfoProviderSuite is a test suite for testing the Provider.
type ExecutionResultInfoProviderSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	params   *protocol.Params
	log      zerolog.Logger

	receipts *storagemock.ExecutionReceipts

	rootBlock       *flow.Block
	rootBlockResult *flow.ExecutionResult
}

func TestExecutionResultInfoProvider(t *testing.T) {
	suite.Run(t, new(ExecutionResultInfoProviderSuite))
}

// SetupTest initializes the test suite with mock state and receipts storage.
func (suite *ExecutionResultInfoProviderSuite) SetupTest() {
	t := suite.T()
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = protocol.NewState(t)
	suite.snapshot = protocol.NewSnapshot(t)
	suite.params = protocol.NewParams(t)
	suite.receipts = storagemock.NewExecutionReceipts(t)

	suite.rootBlock = unittest.BlockFixture()
	rootBlockID := suite.rootBlock.ID()
	suite.rootBlockResult = unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(rootBlockID))
	// This will be used just for the root block
	suite.snapshot.On("SealedResult").Return(suite.rootBlockResult, nil, nil).Maybe()
	suite.state.On("SealedResult", rootBlockID).Return(flow.ExecutionReceiptList{}).Maybe()
	suite.params.On("SporkRootBlock").Return(suite.rootBlock)
	suite.state.On("Params").Return(suite.params)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("AtBlockID", mock.Anything).Return(suite.snapshot).Maybe()
}

func (suite *ExecutionResultInfoProviderSuite) createProvider(
	preferredExecutors flow.IdentifierList,
	operatorCriteria optimistic_sync.Criteria,
) *Provider {
	return NewExecutionResultInfoProvider(
		suite.log,
		suite.state,
		suite.receipts,
		NewExecutionNodeSelector(preferredExecutors, operatorCriteria.RequiredExecutors),
		operatorCriteria,
	)
}

// setupIdentitiesMock sets up the mock for identity-related calls.
func (suite *ExecutionResultInfoProviderSuite) setupIdentitiesMock(allExecutionNodes flow.IdentityList) {
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return allExecutionNodes.Filter(filter)
		},
		func(flow.IdentityFilter[flow.Identity]) error { return nil },
	)
}

// TestExecutionResultProvider tests the main ExecutionResult function with various scenarios.
func (suite *ExecutionResultInfoProviderSuite) TestExecutionResultProvider() {
	totalReceipts := 5
	block := unittest.BlockFixture()

	// generate execution node identities for each receipt
	allExecutionNodes := unittest.IdentityListFixture(
		totalReceipts,
		unittest.WithRole(flow.RoleExecution),
	)

	// create two different execution results to test agreement logic
	executionResult := unittest.ExecutionResultFixture()

	suite.Run(
		"query with client required executors", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

			receipts := make(flow.ExecutionReceiptList, totalReceipts)
			for i := 0; i < totalReceipts; i++ {
				r := unittest.ReceiptForBlockFixture(block)
				r.ExecutorID = allExecutionNodes[i].NodeID
				r.ExecutionResult = *executionResult
				receipts[i] = r
			}

			suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
			suite.setupIdentitiesMock(allExecutionNodes)

			// Require specific executors (first two nodes)
			requiredExecutors := allExecutionNodes[0:2].NodeIDs()

			query, err := provider.ExecutionResultInfo(
				block.ID(), optimistic_sync.Criteria{
					AgreeingExecutorsCount: 2,
					RequiredExecutors:      requiredExecutors,
				},
			)
			suite.Require().NoError(err)

			suite.Assert().ElementsMatch(requiredExecutors, query.ExecutionNodes.NodeIDs())
		},
	)

	suite.Run(
		"successful query with different block results", func() {
			requiredExecutors := allExecutionNodes[0:3].NodeIDs()

			provider := suite.createProvider(
				flow.IdentifierList{}, optimistic_sync.Criteria{
					RequiredExecutors: requiredExecutors,
				},
			)

			otherResult := unittest.ExecutionResultFixture()
			// Create 3 receipts with the same result (executionResult) and 2 with a different result (otherResult)
			receipts := make(flow.ExecutionReceiptList, totalReceipts)
			for i := 0; i < 3; i++ {
				r := unittest.ReceiptForBlockFixture(block)
				r.ExecutorID = allExecutionNodes[i].NodeID
				r.ExecutionResult = *executionResult
				receipts[i] = r
			}
			for i := 3; i < totalReceipts; i++ {
				r := unittest.ReceiptForBlockFixture(block)
				r.ExecutorID = allExecutionNodes[i].NodeID
				r.ExecutionResult = *otherResult
				receipts[i] = r
			}

			suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
			suite.setupIdentitiesMock(allExecutionNodes)

			res, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{})
			suite.Require().NoError(err)

			suite.Require().Equal(executionResult.ID(), res.ExecutionResultID)
			suite.Assert().ElementsMatch(requiredExecutors, res.ExecutionNodes.NodeIDs())
		},
	)

	suite.Run(
		"insufficient agreeing executors returns error", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

			// Create a fresh block for this test to ensure proper isolation
			insufficientBlock := unittest.BlockFixture()

			// Create a scenario where we have receipts but no result has enough agreeing executors

			// Create only 1 receipt with 1 execution result
			r := unittest.ReceiptForBlockFixture(insufficientBlock)
			r.ExecutorID = allExecutionNodes[0].NodeID
			r.ExecutionResult = *unittest.ExecutionResultFixture()
			receipts := flow.ExecutionReceiptList{
				r,
			}

			// Set up a separate mock call for this specific block
			suite.receipts.On("ByBlockID", insufficientBlock.ID()).Return(receipts, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

			_, err := provider.ExecutionResultInfo(
				insufficientBlock.ID(), optimistic_sync.Criteria{
					AgreeingExecutorsCount: 2,
					RequiredExecutors:      allExecutionNodes[0:1].NodeIDs(),
				},
			)
			suite.Require().Error(err)

			suite.Assert().True(common.IsInsufficientExecutionReceipts(err))
		},
	)

	suite.Run(
		"required executors not found returns error", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})
			receipts := make(flow.ExecutionReceiptList, totalReceipts)
			for i := 0; i < totalReceipts; i++ {
				r := unittest.ReceiptForBlockFixture(block)
				r.ExecutorID = allExecutionNodes[i].NodeID
				r.ExecutionResult = *executionResult
				receipts[i] = r
			}

			suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
			suite.setupIdentitiesMock(allExecutionNodes)

			// Require executors that didn't produce any receipts
			_, err := provider.ExecutionResultInfo(
				block.ID(), optimistic_sync.Criteria{
					RequiredExecutors: unittest.IdentityListFixture(
						2,
						unittest.WithRole(flow.RoleExecution),
					).NodeIDs(),
				},
			)
			suite.Require().Error(err)

			suite.Assert().True(common.IsInsufficientExecutionReceipts(err))
		},
	)

	suite.Run("execution result provider recognizes fork switch", func() {
		preferredExecutors := flow.IdentifierList{}
		operatorCriteria := optimistic_sync.Criteria{
			AgreeingExecutorsCount: 1,
		}
		provider := suite.createProvider(preferredExecutors, operatorCriteria)

		// set up 2 executors that produce different execution results
		block := unittest.BlockFixture()
		baseExecutionResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))

		// fork 1
		executionResult1 := unittest.ExecutionResultFixture()
		executionResult1.PreviousResultID = baseExecutionResult.ID()

		// fork 2
		executionResult2 := unittest.ExecutionResultFixture()
		executionResult2.PreviousResultID = baseExecutionResult.ID()

		executors := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
		receipts := make(flow.ExecutionReceiptList, 2)

		r1 := unittest.ReceiptForBlockFixture(block)
		r1.ExecutorID = executors[0].NodeID
		r1.ExecutionResult = *executionResult1
		receipts[0] = r1

		r2 := unittest.ReceiptForBlockFixture(block)
		r2.ExecutorID = executors[1].NodeID
		r2.ExecutionResult = *executionResult2
		receipts[1] = r2

		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(receipts, nil)

		// request execution result from the first executor
		suite.snapshot.
			On("Identities", mock.Anything).
			Return(flow.IdentityList{executors[0]}, nil).
			Once()

		result1, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{
			RequiredExecutors:       flow.IdentifierList{executors[0].NodeID},
			ParentExecutionResultID: baseExecutionResult.ID(),
		})
		suite.Require().NoError(err)
		suite.Require().Equal(executionResult1.ID(), result1.ExecutionResultID)

		// now request the second executor's result (also a child of baseExecutionResult),
		// but require that it descends from result1; since it's on a different fork, no match should be found.
		suite.snapshot.
			On("Identities", mock.Anything).
			Return(flow.IdentityList{executors[1]}, nil).
			Once()

		result2, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{
			RequiredExecutors:       flow.IdentifierList{executors[1].NodeID},
			ParentExecutionResultID: result1.ExecutionResultID,
		})
		suite.Require().ErrorContains(err, "failed to find result")
		suite.Require().Empty(result2)
	})
}

// TestRootBlockHandling tests the special case handling for root blocks.
func (suite *ExecutionResultInfoProviderSuite) TestRootBlockHandling() {
	allExecutionNodes := unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleExecution))
	suite.setupIdentitiesMock(allExecutionNodes)

	suite.Run(
		"root block returns execution nodes without execution result", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

			query, err := provider.ExecutionResultInfo(
				suite.rootBlock.ID(),
				optimistic_sync.Criteria{},
			)
			suite.Require().NoError(err)

			suite.Assert().Equal(suite.rootBlockResult.ID(), query.ExecutionResultID)
			suite.Assert().Len(query.ExecutionNodes.NodeIDs(), defaultMaxNodesCnt)
			suite.Assert().Subset(allExecutionNodes.NodeIDs(), query.ExecutionNodes.NodeIDs())
		},
	)

	suite.Run(
		"root block with required executors", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

			requiredExecutors := allExecutionNodes[0:2].NodeIDs()
			criteria := optimistic_sync.Criteria{
				AgreeingExecutorsCount: 1,
				RequiredExecutors:      requiredExecutors,
			}

			query, err := provider.ExecutionResultInfo(suite.rootBlock.ID(), criteria)
			suite.Require().NoError(err)

			suite.Assert().Equal(suite.rootBlockResult.ID(), query.ExecutionResultID)
			suite.Assert().ElementsMatch(query.ExecutionNodes.NodeIDs(), requiredExecutors)
		},
	)
}

// TestPreferredAndRequiredExecutionNodes tests the interaction with preferred and required execution nodes.
func (suite *ExecutionResultInfoProviderSuite) TestPreferredAndRequiredExecutionNodes() {
	block := unittest.BlockFixture()
	allExecutionNodes := unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleExecution))
	executionResult := unittest.ExecutionResultFixture()

	numReceipts := 6
	// Create receipts from the first `numReceipts` execution nodes
	receipts := make(flow.ExecutionReceiptList, numReceipts)
	for i := 0; i < numReceipts; i++ {
		r := unittest.ReceiptForBlockFixture(block)
		r.ExecutorID = allExecutionNodes[i].NodeID
		r.ExecutionResult = *executionResult
		receipts[i] = r
	}

	suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)
	suite.setupIdentitiesMock(allExecutionNodes)

	suite.Run(
		"with default optimistic_sync.Criteria", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

			// optimistic_sync.Criteria are empty to use operator defaults
			query, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{})
			suite.Require().NoError(err)

			expectedExecutors := allExecutionNodes[0:3].NodeIDs()
			actualExecutors := query.ExecutionNodes.NodeIDs()

			suite.Assert().Len(actualExecutors, defaultMaxNodesCnt)
			suite.Assert().ElementsMatch(expectedExecutors, actualExecutors)
		},
	)

	suite.Run(
		"with operator preferred executors", func() {
			provider := suite.createProvider(
				allExecutionNodes[1:5].NodeIDs(),
				optimistic_sync.Criteria{},
			)

			// optimistic_sync.Criteria are empty to use operator defaults
			query, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{})
			suite.Require().NoError(err)

			actualExecutors := query.ExecutionNodes.NodeIDs()

			suite.Assert().ElementsMatch(
				provider.executionNodeSelector.preferredENIdentifiers,
				actualExecutors,
			)
		},
	)

	suite.Run(
		"with operator required executors", func() {
			provider := suite.createProvider(
				flow.IdentifierList{}, optimistic_sync.Criteria{
					RequiredExecutors: allExecutionNodes[5:8].NodeIDs(),
				},
			)

			// optimistic_sync.Criteria are empty to use operator defaults
			query, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{})
			suite.Require().NoError(err)

			actualExecutors := query.ExecutionNodes.NodeIDs()

			// Just one required executor contains the result
			expectedExecutors := provider.executionNodeSelector.requiredENIdentifiers[0:1]

			suite.Assert().ElementsMatch(expectedExecutors, actualExecutors)
		},
	)

	suite.Run(
		"with both: operator preferred & required executors", func() {
			provider := suite.createProvider(
				allExecutionNodes[0:1].NodeIDs(), optimistic_sync.Criteria{
					RequiredExecutors: allExecutionNodes[3:6].NodeIDs(),
				},
			)

			// optimistic_sync.Criteria are empty to use operator defaults
			query, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{})
			suite.Require().NoError(err)

			// `preferredENIdentifiers` contain 1 executor, that is not enough, so the logic will get 2 executors from `requiredENIdentifiers` to fill `defaultMaxNodesCnt` executors.
			expectedExecutors := append(
				provider.executionNodeSelector.preferredENIdentifiers,
				provider.executionNodeSelector.requiredENIdentifiers[0:2]...,
			)
			actualExecutors := query.ExecutionNodes.NodeIDs()

			suite.Assert().Len(actualExecutors, defaultMaxNodesCnt)
			suite.Assert().ElementsMatch(expectedExecutors, actualExecutors)
		},
	)

	suite.Run(
		"with client preferred executors", func() {
			provider := suite.createProvider(
				allExecutionNodes[0:1].NodeIDs(), optimistic_sync.Criteria{
					RequiredExecutors: allExecutionNodes[2:4].NodeIDs(),
				},
			)

			userCriteria := optimistic_sync.Criteria{
				RequiredExecutors: allExecutionNodes[5:6].NodeIDs(),
			}

			query, err := provider.ExecutionResultInfo(block.ID(), userCriteria)
			suite.Require().NoError(err)

			suite.Assert().ElementsMatch(
				userCriteria.RequiredExecutors,
				query.ExecutionNodes.NodeIDs(),
			)
		},
	)
}
