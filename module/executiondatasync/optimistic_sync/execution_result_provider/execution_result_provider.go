package execution_result_provider

import (
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ExecutionResultProvider is a container for elements required to retrieve
// execution results and execution node identities for a given block ID based on specified criteria.
type ExecutionResultProvider struct {
	log zerolog.Logger

	executionReceipts storage.ExecutionReceipts
	state             protocol.State

	executionNodes *ExecutionNodesSelector

	rootBlockID     flow.Identifier
	rootBlockResult *flow.ExecutionResult

	baseCriteria optimistic_sync.Criteria
}

var _ optimistic_sync.ExecutionResultInfoProvider = (*ExecutionResultProvider)(nil)

// NewExecutionResultProvider creates and returns a new instance of
// ExecutionResultProvider.
//
// No errors are expected during normal operations
func NewExecutionResultProvider(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	executionReceipts storage.ExecutionReceipts,
	executionNodes *ExecutionNodesSelector,
	operatorCriteria optimistic_sync.Criteria,
) (*ExecutionResultProvider, error) {
	// Root block ID and result should not change and could be cached.
	sporkRootBlockHeight := state.Params().SporkRootBlockHeight()
	rootBlockID, err := headers.BlockIDByHeight(sporkRootBlockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block ID by height: %w", err)
	}

	rootBlockResult, _, err := state.AtBlockID(rootBlockID).SealedResult()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve root block result: %w", err)
	}

	return &ExecutionResultProvider{
		log:               log.With().Str("module", "execution_result_query").Logger(),
		executionReceipts: executionReceipts,
		state:             state,
		executionNodes:    executionNodes,
		rootBlockID:       rootBlockID,
		rootBlockResult:   rootBlockResult,
		baseCriteria:      optimistic_sync.DefaultCriteria.OverrideWith(operatorCriteria),
	}, nil
}

// ExecutionResult retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria.
//
// Expected errors during normal operations:
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
func (e *ExecutionResultProvider) ExecutionResult(blockID flow.Identifier, criteria optimistic_sync.Criteria) (*optimistic_sync.ExecutionResultInfo, error) {
	executorIdentities, err := e.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve execution IDs for root block: %w", err)
	}

	// if the block ID is the root block, then use the root ExecutionResult and skip the receipt
	// check since there will not be any.
	if e.rootBlockID == blockID {
		subsetENs, err := e.executionNodes.SelectExecutionNodes(executorIdentities, criteria.RequiredExecutors)
		if err != nil {
			return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", e.rootBlockID, err)
		}

		return &optimistic_sync.ExecutionResultInfo{
			ExecutionResult: e.rootBlockResult,
			ExecutionNodes:  subsetENs,
		}, nil
	}

	result, executorIDs, err := e.findResultAndExecutors(blockID, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to find result and executors for block ID %v: %w", blockID, err)
	}

	executors := executorIdentities.Filter(filter.HasNodeID[flow.Identity](executorIDs...))
	subsetENs, err := e.executionNodes.SelectExecutionNodes(executors, criteria.RequiredExecutors)
	if err != nil {
		return nil, fmt.Errorf("failed to choose execution nodes for block ID %v: %w", blockID, err)
	}

	if len(subsetENs) == 0 {
		// this is unexpected, and probably indicates there is a bug.
		// There are only three ways that SelectExecutionNodes can return an empty list:
		//   1. there are no executors for the result
		//   2. none of the user's required executors are in the executor list
		//   3. none of the operator's required executors are in the executor list
		// None of these are possible since there must be at least one AgreeingExecutorsCount. If the
		// criteria is met, then there must be at least one acceptable executor. If this is not true,
		// then the criteria check must fail.
		return nil, fmt.Errorf("no execution nodes found for result %v (blockID: %v): %w", result.ID(), blockID, err)
	}

	return &optimistic_sync.ExecutionResultInfo{
		ExecutionResult: result,
		ExecutionNodes:  subsetENs,
	}, nil
}

// findResultAndExecutors returns a query response for a given block ID.
// The result must match the provided criteria and have at least one acceptable executor. If multiple
// results are found, then the result with the most executors is returned.
//
// Expected errors during normal operations:
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
func (e *ExecutionResultProvider) findResultAndExecutors(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (*flow.ExecutionResult, flow.IdentifierList, error) {
	type result struct {
		result   *flow.ExecutionResult
		receipts flow.ExecutionReceiptList
	}

	criteria = e.baseCriteria.OverrideWith(criteria)

	// Note: this will return an empty slice with no error if no receipts are found.
	allReceipts, err := e.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	// find all results that match the criteria and have at least one acceptable executor
	results := make([]result, 0)
	for _, executionReceiptList := range allReceipts.GroupByResultID() {
		executorGroup := executionReceiptList.GroupByExecutorID()
		if isExecutorGroupMeetingCriteria(executorGroup, criteria) {
			results = append(results, result{
				result:   &executionReceiptList[0].ExecutionResult,
				receipts: executionReceiptList,
			})
		}
	}

	if len(results) == 0 {
		return nil, nil, common.NewInsufficientExecutionReceipts(blockID, 0)
	}

	// sort results by the number of execution nodes in descending order
	sort.Slice(results, func(i, j int) bool {
		return len(results[i].receipts) > len(results[j].receipts)
	})

	executorIDs := getExecutorIDs(results[0].receipts)
	return results[0].result, executorIDs, nil
}

// isExecutorGroupMeetingCriteria checks if an executor group meets the specified criteria for execution receipts matching.
func isExecutorGroupMeetingCriteria(executorGroup flow.ExecutionReceiptGroupedList, criteria optimistic_sync.Criteria) bool {
	if uint(len(executorGroup)) < criteria.AgreeingExecutorsCount {
		return false
	}

	if len(criteria.RequiredExecutors) > 0 {
		hasRequiredExecutor := false
		for _, requiredExecutor := range criteria.RequiredExecutors {
			if _, ok := executorGroup[requiredExecutor]; ok {
				hasRequiredExecutor = true
				break
			}
		}
		if !hasRequiredExecutor {
			return false
		}
	}

	// TODO: Implement the `ResultInFork` check here, which iteratively checks ancestors to determine if
	//       the current result's fork includes the requested result. https://github.com/onflow/flow-go/issues/7587

	return true
}

// getExecutorIDs extracts unique executor node IDs from a list of execution receipts.
// It groups receipts by executor ID and returns all unique executor identifiers.
func getExecutorIDs(receipts flow.ExecutionReceiptList) flow.IdentifierList {
	receiptGroupedByExecutorID := receipts.GroupByExecutorID()

	executorIDs := make(flow.IdentifierList, 0, len(receiptGroupedByExecutorID))
	for executorID := range receiptGroupedByExecutorID {
		executorIDs = append(executorIDs, executorID)
	}

	return executorIDs
}
