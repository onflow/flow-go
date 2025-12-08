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
	// This will be used just for the root block
	suite.snapshot.On("SealedResult").Return(suite.rootBlockResult, nil, nil).Maybe()
	suite.state.On("SealedResult", rootBlockID).Return(flow.ExecutionReceiptList{}).Maybe()
	suite.params.On("SporkRootBlock").Return(suite.rootBlock)
	suite.state.On("Params").Return(suite.params)
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
		suite.headers,
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
	).Once()
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

			suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil).Once()
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
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

			suite.setupIdentitiesMock(allExecutionNodes)
			suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil).Once()
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()

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

			// Set up a separate mock call for this specific block
			suite.receipts.On("ByBlockID", insufficientBlock.ID()).Return(receipts, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

			result, err := provider.ExecutionResultInfo(
				insufficientBlock.ID(), optimistic_sync.Criteria{
					AgreeingExecutorsCount: 2,
					RequiredExecutors:      allExecutionNodes[0:1].NodeIDs(),
				},
			)
			suite.Require().Error(err)
			suite.Require().Nil(result)
			suite.Assert().True(common.IsInsufficientExecutionReceipts(err))
		},
	)

	suite.Run(
		"required executors not found returns error", func() {
			provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})
			receipts := make(flow.ExecutionReceiptList, totalReceipts)
			for i := 0; i < totalReceipts; i++ {
				r := unittest.ReceiptForBlockFixture(block)
				r.ExecutorID = allExecutionNodes[0].NodeID
				r.ExecutionResult = *executionResult
				receipts[i] = r
			}

			suite.receipts.On("ByBlockID", block.ID()).Return(receipts, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

			// Require executors that didn't produce any receipts
			result, err := provider.ExecutionResultInfo(
				block.ID(), optimistic_sync.Criteria{
					RequiredExecutors: allExecutionNodes[1:2].NodeIDs(),
				},
			)
			suite.Require().Error(err)
			suite.Require().Nil(result)
			suite.Assert().True(common.IsInsufficientExecutionReceipts(err))
		},
	)

	suite.Run("required executors count is greater than available executors count returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

		// setup specific executors (first two nodes)
		suite.setupIdentitiesMock(allExecutionNodes[0:2])
		requiredExecutors := allExecutionNodes.NodeIDs()

		query, err := provider.ExecutionResultInfo(
			block.ID(), optimistic_sync.Criteria{
				AgreeingExecutorsCount: 2,
				RequiredExecutors:      requiredExecutors,
			},
		)
		suite.Require().Error(err)
		suite.Require().Nil(query)
		suite.Require().True(optimistic_sync.IsRequiredExecutorsCountExceededError(err))
	})

	suite.Run("agreeing executors count is greater than available executors count returns error", func() {
		provider := suite.createProvider(flow.IdentifierList{}, optimistic_sync.Criteria{})

		suite.setupIdentitiesMock(allExecutionNodes)
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

		suite.setupIdentitiesMock(allExecutionNodes)

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

	suite.Run("criteria not met on sealed block returns error", func() {
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
		suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
		suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
		suite.setupIdentitiesMock(allExecutionNodes)

		// Require all executors, but only one produces receipts
		query, err := provider.ExecutionResultInfo(
			block.ID(), optimistic_sync.Criteria{
				AgreeingExecutorsCount: 1,
				RequiredExecutors:      allExecutionNodes.NodeIDs(),
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

	suite.Run(
		"root block returns execution nodes without execution result", func() {
			suite.setupIdentitiesMock(allExecutionNodes)
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
			suite.setupIdentitiesMock(allExecutionNodes)
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

	suite.Run(
		"with default optimistic_sync.Criteria", func() {
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

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
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

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
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

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
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

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
			suite.headers.On("ByBlockID", block.ID()).Return(block.ToHeader(), nil).Once()
			suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
			suite.snapshot.On("Head").Return(func() *flow.Header { return block.ToHeader() }, nil).Once()
			suite.setupIdentitiesMock(allExecutionNodes)

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
