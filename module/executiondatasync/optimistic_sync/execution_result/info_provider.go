package execution_result

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

// Provider is a container for elements required to retrieve
// execution results and execution node identities for a given block ID based on specified criteria.
type Provider struct {
	log zerolog.Logger

	executionReceipts storage.ExecutionReceipts
	headers           storage.Headers
	state             protocol.State
	rootBlockID       flow.Identifier
	executionNodes    *ExecutionNodeSelector
	baseCriteria      optimistic_sync.Criteria
}

var _ optimistic_sync.ExecutionResultInfoProvider = (*Provider)(nil)

// NewExecutionResultInfoProvider creates and returns a new instance of Provider.
func NewExecutionResultInfoProvider(
	log zerolog.Logger,
	state protocol.State,
	executionReceipts storage.ExecutionReceipts,
	headers storage.Headers,
	executionNodes *ExecutionNodeSelector,
	operatorCriteria optimistic_sync.Criteria,
) *Provider {
	return &Provider{
		log:               log.With().Str("module", "execution_result_info").Logger(),
		executionReceipts: executionReceipts,
		headers:           headers,
		state:             state,
		executionNodes:    executionNodes,
		rootBlockID:       state.Params().SporkRootBlock().ID(),
		baseCriteria:      optimistic_sync.DefaultCriteria.OverrideWith(operatorCriteria),
	}
}

// ExecutionResultInfo retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria.
//
// Expected errors during normal operations:
//   - [common.InsufficientExecutionReceipts]: Found insufficient receipts for given block ID.
//   - [storage.ErrNotFound]: If the data was not found.
//   - [optimistic_sync.RequiredExecutorsCountExceededError]: Required executor IDs count exceeds available executors.
//   - [optimistic_sync.AgreeingExecutorsCountExceededError]: Agreeing executors count exceeds available executors.
//   - [optimistic_sync.UnknownRequiredExecutorError]: A required executor ID is not in the available set.
func (e *Provider) ExecutionResultInfo(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (*optimistic_sync.ExecutionResultInfo, error) {
	executorIdentities, err := e.state.AtBlockID(blockID).Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve execution IDs: %w", err)
	}

	err = e.validateCriteria(criteria, executorIdentities)
	if err != nil {
		return nil, fmt.Errorf("invalid required executors: %w", err)
	}

	// if the block ID is the root block, then use the root ExecutionResult and skip the receipt
	// check since there will not be any.
	if e.rootBlockID == blockID {
		subsetENs, err := e.executionNodes.SelectExecutionNodes(executorIdentities, criteria.RequiredExecutors)
		if err != nil {
			return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", e.rootBlockID, err)
		}

		rootBlockResult, _, err := e.state.AtBlockID(e.rootBlockID).SealedResult()
		if err != nil {
			// if the node was bootstrapped from a block after the spork root block, then the root
			// block's result will not be present.
			return nil, fmt.Errorf("failed to retrieve root block result: %w", err)
		}

		return &optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: rootBlockResult.ID(),
			ExecutionNodes:    subsetENs,
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

	// If criteria cannot be met, return an error
	if len(subsetENs) < len(criteria.RequiredExecutors) {
		return nil, optimistic_sync.NewCriteriaNotMetError(blockID)
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

	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get header by block ID %v: %w", blockID, err)
	}
	// Lookup the finalized block ID at the height of the requested block
	blockIDFinalized, err := e.headers.BlockIDByHeight(header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup finalized block ID at height %d: %w", header.Height, err)
	}
	// If the requested block conflicts with finalized block, return error
	if blockIDFinalized != blockID {
		return nil, optimistic_sync.NewBlockFinalityMismatchError(blockID, blockIDFinalized)
	}

	return &optimistic_sync.ExecutionResultInfo{
		ExecutionResultID: result.ID(),
		ExecutionNodes:    subsetENs,
	}, nil
}

// validateCriteria verifies that the provided optimistic sync criteria can be
// satisfied by the currently available execution nodes.
//
// The validation ensures that the required executor IDs do not exceed the number
// of available executors, that the requested AgreeingExecutorsCount is feasible,
// and that every required executor ID is present in the available set.
//
// Expected errors during normal operations:
//   - [optimistic_sync.RequiredExecutorsCountExceededError]: Required executor IDs count exceeds available executors.
//   - [optimistic_sync.AgreeingExecutorsCountExceededError]: Agreeing executors count exceeds available executors.
//   - [optimistic_sync.UnknownRequiredExecutorError]: A required executor ID is not in the available set.
func (e *Provider) validateCriteria(
	criteria optimistic_sync.Criteria,
	availableExecutors flow.IdentityList,
) error {
	requiredExecutors := criteria.RequiredExecutors
	if len(availableExecutors) < len(requiredExecutors) {
		return optimistic_sync.NewRequiredExecutorsCountExceededError(
			len(requiredExecutors),
			len(availableExecutors),
		)
	}
	if uint(len(availableExecutors)) < criteria.AgreeingExecutorsCount {
		return optimistic_sync.NewAgreeingExecutorsCountExceededError(
			criteria.AgreeingExecutorsCount,
			len(availableExecutors),
		)
	}

	lookup := availableExecutors.Lookup()
	for _, executorID := range requiredExecutors {
		if _, ok := lookup[executorID]; !ok {
			return optimistic_sync.NewUnknownRequiredExecutorError(executorID)
		}
	}

	return nil
}

// findResultAndExecutors returns a query response for a given block ID.
// The result must match the provided criteria and have at least one acceptable executor. If multiple
// results are found, then the result with the most executors is returned.
//
// Expected errors during normal operations:
//   - [common.InsufficientExecutionReceipts]: Found insufficient receipts for given block ID.
func (e *Provider) findResultAndExecutors(
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
func isExecutorGroupMeetingCriteria(
	executorGroup flow.ExecutionReceiptGroupedList,
	criteria optimistic_sync.Criteria,
) bool {
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
