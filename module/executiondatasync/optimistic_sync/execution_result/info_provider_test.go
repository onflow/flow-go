package execution_result

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// ExecutionResultInfoProviderSuite is a test suite for testing the Provider.
type ExecutionResultInfoProviderSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	params   *protocol.Params
	log      zerolog.Logger

	receipts *storagemock.ExecutionReceipts
	headers  *storagemock.Headers

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
	suite.headers = storagemock.NewHeaders(t)
	suite.rootBlock = unittest.BlockFixture()
	rootBlockID := suite.rootBlock.ID()
	suite.rootBlockResult = unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(rootBlockID))
}

func (suite *ExecutionResultInfoProviderSuite) createProvider(
	preferredExecutors flow.IdentifierList,
	operatorCriteria optimistic_sync.Criteria,
) *Provider {
	return NewExecutionResultInfoProvider(
		suite.log,
		suite.state,
		suite.receipts,
		suite.headers,
		NewExecutionNodeSelector(preferredExecutors, operatorCriteria.RequiredExecutors),
		operatorCriteria,
	)
}

// TestExecutionResultQuery tests the main ExecutionResult function with various scenarios.
func (suite *ExecutionResultInfoProviderSuite) TestExecutionResultQuery() {
	totalReceipts := 5
	block := unittest.BlockFixture()

	// generate execution node identities for each receipt
	allExecutionNodes := unittest.IdentityListFixture(
		totalReceipts,
		unittest.WithRole(flow.RoleExecution),
	)

	// create two different execution results to test agreement logic
	executionResult := unittest.ExecutionResultFixture()

	suite.snapshot.
		On("Identities", mock.Anything).
		Return(func(filter flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
			return allExecutionNodes.Filter(filter), nil
		}).
		Times(7) // for each subtest in this test case

	suite.state.
		On("Params").
		Return(suite.params).
		Times(7) // for each subtest in this test case

	suite.params.
		On("SporkRootBlock").
		Return(suite.rootBlock).
		Times(7) // for each subtest in this test case

	suite.state.
		On("AtBlockID", mock.Anything).
		Return(suite.snapshot).
		Times(7) // for each subtest in this test case

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

			suite.receipts.
				On("ByBlockID", block.ID()).
				Return(receipts, nil).
				Once()

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

			suite.receipts.
				On("ByBlockID", block.ID()).
				Return(receipts, nil).
				Once()

			query, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{})
			suite.Require().NoError(err)

			suite.Require().Equal(executionResult.ID(), query.ExecutionResultID)
			suite.Assert().ElementsMatch(requiredExecutors, query.ExecutionNodes.NodeIDs())
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

			suite.receipts.
				On("ByBlockID", insufficientBlock.ID()).
				Return(receipts, nil).
				Once()

			suite.headers.
				On("ByBlockID", insufficientBlock.ID()).
				Return(insufficientBlock.ToHeader(), nil).
				Once()
			suite.headers.
				On("BlockIDByHeight", insufficientBlock.Height).
				Return(unittest.IdentifierFixture(), nil).
				Once()

			_, err := provider.ExecutionResultInfo(
				insufficientBlock.ID(), optimistic_sync.Criteria{
					AgreeingExecutorsCount: 2,
					RequiredExecutors:      allExecutionNodes[0:1].NodeIDs(),
				},
			)
			suite.Require().ErrorIs(err, optimistic_sync.ErrNotEnoughAgreeingExecutors)
		},
	)

	suite.Run(
		"required executors not found returns error", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})
			// only some of the available executors produce receipts,
			// so we can require an available executor that did not produce a receipt
			receipts := make(flow.ExecutionReceiptList, totalReceipts-1)
			for i := 0; i < totalReceipts-1; i++ {
				r := unittest.ReceiptForBlockFixture(block)
				r.ExecutorID = allExecutionNodes[i].NodeID
				r.ExecutionResult = *executionResult
				receipts[i] = r
			}

			suite.receipts.
				On("ByBlockID", block.ID()).
				Return(receipts, nil).
				Once()

			// Require an available executor that didn't produce any receipts for the chosen result
			_, err := provider.ExecutionResultInfo(
				block.ID(), optimistic_sync.Criteria{
					RequiredExecutors: allExecutionNodes[totalReceipts-1 : totalReceipts].NodeIDs(),
				},
			)
			suite.Require().ErrorIs(err, optimistic_sync.ErrRequiredExecutorNotFound)
		},
	)

	suite.Run("agreeing executors count is greater than available executors count returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

		requiredExecutors := allExecutionNodes.NodeIDs()

		query, err := provider.ExecutionResultInfo(
			block.ID(), optimistic_sync.Criteria{
				AgreeingExecutorsCount: uint(len(allExecutionNodes) + 1),
				RequiredExecutors:      requiredExecutors,
			},
		)
		suite.Require().Error(err)
		suite.Require().Nil(query)
		suite.Require().True(optimistic_sync.IsAgreeingExecutorsCountExceededError(err))
	})

	suite.Run("unknown required executor returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

		unknownExecutorID := unittest.IdentifierFixture()
		requiredExecutors := allExecutionNodes[0:1].NodeIDs()
		requiredExecutors = append(requiredExecutors, unknownExecutorID)

		query, err := provider.ExecutionResultInfo(
			block.ID(), optimistic_sync.Criteria{
				AgreeingExecutorsCount: 2,
				RequiredExecutors:      requiredExecutors,
			},
		)
		suite.Require().Error(err)
		suite.Require().Nil(query)
		suite.Require().True(optimistic_sync.IsUnknownRequiredExecutorError(err))
	})

	suite.Run("criteria not met returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

		receipts := make(flow.ExecutionReceiptList, totalReceipts)
		for i := 0; i < totalReceipts; i++ {
			r := unittest.ReceiptForBlockFixture(block)
			r.ExecutorID = allExecutionNodes[0].NodeID
			r.ExecutionResult = *executionResult
			receipts[i] = r
		}
		suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil).Once()
		suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
		suite.headers.On("BlockIDByHeight", block.Height).Return(block.ID(), nil).Once()
		suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
		suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()

		// require more agreeing executors than available for the chosen result
		query, err := provider.ExecutionResultInfo(
			block.ID(), optimistic_sync.Criteria{
				AgreeingExecutorsCount: 2,
			},
		)
		suite.Require().Error(err)
		suite.Require().Nil(query)
		suite.Require().True(optimistic_sync.IsCriteriaNotMetError(err))
	})
}

// TestRootBlockHandling tests the special case handling for root blocks.
func (suite *ExecutionResultInfoProviderSuite) TestRootBlockHandling() {
	allExecutionNodes := unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleExecution))
	suite.snapshot.
		On("Identities", mock.Anything).
		Return(func(filter flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
			return allExecutionNodes.Filter(filter), nil
		}).
		Times(2) // for each subtest

	// expected to be called once in each subtest
	suite.state.On("Params").Return(suite.params).Times(2)
	suite.params.On("SporkRootBlock").Return(suite.rootBlock).Times(2)
	suite.snapshot.On("SealedResult").Return(suite.rootBlockResult, nil, nil).Times(2)

	// expected to be called twice once in each subtest
	suite.state.On("AtBlockID", suite.rootBlock.ID()).Return(suite.snapshot).Times(4)

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

	suite.snapshot.
		On("Identities", mock.Anything).
		Return(func(filter flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
			return allExecutionNodes.Filter(filter), nil
		}).
		Times(5) // for each subtest

	// expected to be called once in each subtest
	suite.state.On("Params").Return(suite.params).Times(5)
	suite.state.On("AtBlockID", mock.Anything).Return(suite.snapshot).Times(5)
	suite.params.On("SporkRootBlock").Return(suite.rootBlock).Times(5)
	suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil).Times(5)

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
				provider.executionNodes.preferredENIdentifiers,
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
			expectedExecutors := provider.executionNodes.requiredENIdentifiers[0:1]

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
				provider.executionNodes.preferredENIdentifiers,
				provider.executionNodes.requiredENIdentifiers[0:2]...,
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

func (suite *ExecutionResultInfoProviderSuite) TestExecutionResultProviderForkError() {
	preferredExecutors := flow.IdentifierList{}
	operatorCriteria := optimistic_sync.Criteria{
		AgreeingExecutorsCount: 1,
	}

	suite.params.On("SporkRootBlock").Return(suite.rootBlock).Once()
	suite.state.On("Params").Return(suite.params).Once()
	suite.state.On("AtBlockID", mock.Anything).Return(suite.snapshot).Times(2)

	provider := suite.createProvider(preferredExecutors, operatorCriteria)

	g := fixtures.NewGeneratorSuite()

	// set up 2 executors that produce different execution results
	block := g.Blocks().Fixture()
	baseExecutionResult := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlock(block))

	// fork 1 and fork 2, both descend from baseExecutionResult
	executionResult1 := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithPreviousResultID(baseExecutionResult.ID()))
	executionResult2 := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithPreviousResultID(baseExecutionResult.ID()))

	// two execution identities (executors)
	executors := g.Identities().List(2, fixtures.Identity.WithRole(flow.RoleExecution))

	// receipts for both forks, one per executor
	receipt1 := g.ExecutionReceipts().Fixture(
		fixtures.ExecutionReceipt.WithExecutorID(executors[0].NodeID),
		fixtures.ExecutionReceipt.WithExecutionResult(*executionResult1),
	)
	receipt2 := g.ExecutionReceipts().Fixture(
		fixtures.ExecutionReceipt.WithExecutorID(executors[1].NodeID),
		fixtures.ExecutionReceipt.WithExecutionResult(*executionResult2),
	)
	receipts := flow.ExecutionReceiptList{receipt1, receipt2}

	// the first call returns both receipts to select fork1
	suite.receipts.
		On("ByBlockID", block.ID()).
		Return(receipts, nil).
		Once()

	// the second call returns only fork2's receipt to force fork mismatch deterministically
	suite.receipts.
		On("ByBlockID", block.ID()).
		Return(flow.ExecutionReceiptList{receipt2}, nil).
		Once()

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
		Return(flow.IdentityList{executors[0], executors[1]}, nil).
		Once()

	result2, err := provider.ExecutionResultInfo(block.ID(), optimistic_sync.Criteria{
		RequiredExecutors:       flow.IdentifierList{executors[1].NodeID},
		ParentExecutionResultID: result1.ExecutionResultID,
	})
	suite.Require().ErrorIs(err, optimistic_sync.ErrForkAbandoned)
	suite.Require().Empty(result2)
}
