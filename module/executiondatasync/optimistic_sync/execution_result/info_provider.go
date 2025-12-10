package execution_result

import (
	"errors"
	"fmt"
	"sort"

	"github.com/rs/zerolog"

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
	executionNodes *ExecutionNodeSelector,
	operatorCriteria optimistic_sync.Criteria,
) *Provider {
	return &Provider{
		log:               log.With().Str("module", "execution_result_info").Logger(),
		executionReceipts: executionReceipts,
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
//   - [optimistic_sync.ErrBlockNotFound]: If the request is for the spork root block, and the node was bootstrapped
//     from a newer block.
//   - [storage.ErrNotFound]: If the underlying storage does not yet contain data required to fulfill the request
//     (e.g. receipts or result info for the given block ID are not found). This is a benign condition and callers
//     should treat it as "data not available yet" and may retry later.
//   - [optimistic_sync.NotEnoughAgreeingExecutorsError]: If no insufficient receipts are found for given block ID.
//   - [optimistic_sync.ErrForkAbandoned]: If the execution fork of an execution node from which we were getting the
//     execution results was abandoned.
//   - [optimistic_sync.ErrNotEnoughAgreeingExecutors]: If there are not enough execution nodes that produced the
//     execution result.
//   - [optimistic_sync.ErrRequiredExecutorNotFound]: If the criteria's required executor is not in the group of
//     execution nodes that produced the execution result.
func (p *Provider) ExecutionResultInfo(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (*optimistic_sync.ExecutionResultInfo, error) {
	executorIdentities, err := p.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve execution IDs: %w", err)
	}

	// if the block ID is the root block, then use the root ExecutionResult and skip the receipt
	// check since there will not be any.
	if p.rootBlockID == blockID {
		subsetENs, err := p.executionNodes.SelectExecutionNodes(executorIdentities, criteria.RequiredExecutors)
		if err != nil {
			return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", p.rootBlockID, err)
		}

		rootBlockResult, _, err := p.state.AtBlockID(p.rootBlockID).SealedResult()
		if err != nil {
			// if the node was bootstrapped from a block after the spork root block, then the root
			// block's result will not be present.
			return nil, errors.Join(optimistic_sync.ErrBlockNotFound, err)
		}

		return &optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: rootBlockResult.ID(),
			ExecutionNodes:    subsetENs,
		}, nil
	}

	resultID, executorIDs, err := p.findResultAndExecutors(blockID, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to find result and executors for block ID %v: %w", blockID, err)
	}

	executors := executorIdentities.Filter(filter.HasNodeID[flow.Identity](executorIDs...))
	subsetENs, err := p.executionNodes.SelectExecutionNodes(executors, criteria.RequiredExecutors)
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
		return nil, fmt.Errorf("no execution nodes found for result %v (blockID: %v): %w", resultID, blockID, err)
	}

	return &optimistic_sync.ExecutionResultInfo{
		ExecutionResultID: resultID,
		ExecutionNodes:    subsetENs,
	}, nil
}

// findExecutionResultAndExecutors returns a query response for a given block ID.
// The result must match the provided criteria and have at least one acceptable executor. If multiple
// results are found, then the result with the most executors is returned.
//
// Expected errors during normal operations:
//   - [optimistic_sync.ErrForkAbandoned]: If the execution result is in a different fork than the one specified in the criteria.
//   - [optimistic_sync.ErrNotEnoughAgreeingExecutors]: If the group does not have enough agreeing executors.
//   - [optimistic_sync.ErrRequiredExecutorNotFound]: If the required executor is not in the group.
func (p *Provider) findResultAndExecutors(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (flow.Identifier, flow.IdentifierList, error) {
	type resultWithReceipts struct {
		id       flow.Identifier
		receipts flow.ExecutionReceiptList
	}

	criteria = p.baseCriteria.OverrideWith(criteria)

	allReceiptsForBlock, err := p.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	// find all results that match the criteria and have at least one acceptable executor
	matchingResults := make([]resultWithReceipts, 0)
	var lastErr error
	for executionResultID, executionReceiptList := range allReceiptsForBlock.GroupByResultID() {
		result := &executionReceiptList[0].ExecutionResult
		receipts := executionReceiptList.GroupByExecutorID()

		if err := p.isExecutorGroupMeetingCriteria(result, receipts, criteria); err != nil {
			// skip groups that don't meet criteria; remember the last error
			lastErr = err
			continue
		}

		matchingResults = append(matchingResults, resultWithReceipts{
			id:       executionResultID,
			receipts: executionReceiptList,
		})
	}

	if len(matchingResults) == 0 {
		if lastErr != nil {
			return flow.ZeroID, nil, lastErr
		}
		return flow.ZeroID, nil, optimistic_sync.ErrNotEnoughAgreeingExecutors
	}

	// sort matchingResults by the number of execution nodes in descending order
	sort.Slice(matchingResults, func(i, j int) bool {
		return len(matchingResults[i].receipts) > len(matchingResults[j].receipts)
	})

	executorIDs := getExecutorIDs(matchingResults[0].receipts)
	return matchingResults[0].id, executorIDs, nil
}

// isExecutorGroupMeetingCriteria checks if an executor group meets the specified criteria for execution receipts matching.
//
// Expected errors during normal operations:
//   - [optimistic_sync.ErrForkAbandoned]: If the execution result is in a different fork than the one specified in the criteria.
//   - [optimistic_sync.ErrNotEnoughAgreeingExecutors]: If the group does not have enough agreeing executors.
//   - [optimistic_sync.ErrRequiredExecutorNotFound]: If the required executor is not in the group.
func (p *Provider) isExecutorGroupMeetingCriteria(
	executionResult *flow.ExecutionResult,
	executorToExecutionReceipts flow.ExecutionReceiptGroupedList,
	criteria optimistic_sync.Criteria,
) error {
	if uint(len(executorToExecutionReceipts)) < criteria.AgreeingExecutorsCount {
		return optimistic_sync.ErrNotEnoughAgreeingExecutors
	}

	// First, ensure the execution result is in the same execution fork, if any.
	// This avoids returning other errors (like required executor missing) for a fork
	// that we wouldn't consider anyway.
	if criteria.ParentExecutionResultID != flow.ZeroID &&
		executionResult.PreviousResultID != criteria.ParentExecutionResultID {
		return optimistic_sync.ErrForkAbandoned
	}

	// Then, make sure one of the required executors is in the group
	if len(criteria.RequiredExecutors) > 0 {
		hasRequiredExecutor := false
		for _, requiredExecutor := range criteria.RequiredExecutors {
			if _, ok := executorToExecutionReceipts[requiredExecutor]; ok {
				hasRequiredExecutor = true
				break
			}
		}

		if !hasRequiredExecutor {
			return optimistic_sync.ErrRequiredExecutorNotFound
		}
	}

	return nil
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
