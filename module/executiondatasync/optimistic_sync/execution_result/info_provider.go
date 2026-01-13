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

	executionReceipts     storage.ExecutionReceipts
	headers               storage.Headers
	state                 protocol.State
	rootBlockID           flow.Identifier
	executionNodes        *ExecutionNodeSelector
	baseCriteria          optimistic_sync.Criteria
	sealingStatusResolver SealingStatusResolver
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
	sealingStatusResolver SealingStatusResolver,
) *Provider {
	return &Provider{
		log:                   log.With().Str("module", "execution_result_info").Logger(),
		executionReceipts:     executionReceipts,
		headers:               headers,
		state:                 state,
		executionNodes:        executionNodes,
		rootBlockID:           state.Params().SporkRootBlock().ID(),
		baseCriteria:          optimistic_sync.DefaultCriteria.OverrideWith(operatorCriteria),
		sealingStatusResolver: sealingStatusResolver,
	}
}

// ExecutionResultInfo retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria.
//
// Expected errors during normal operations:
//   - [optimistic_sync.ErrBlockBeforeNodeHistory]: If the request is for data before the node's root block.
//   - [optimistic_sync.ErrExecutionResultNotReady]: If criteria cannot be satisfied at the moment.
//   - [optimistic_sync.CriteriaNotMetError]: Returned when the block is already
//     sealed but no execution result can satisfy the provided criteria.
func (p *Provider) ExecutionResultInfo(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (*optimistic_sync.ExecutionResultInfo, error) {
	availableExecutors, err :=
		p.state.AtBlockID(blockID).Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve available executors for the block: %w", err)
	}

	// if the block ID is the root block, then use the root ExecutionResult and skip the receipt
	// check since there will not be any.
	if blockID == p.rootBlockID {
		rootBlockResult, _, err := p.state.AtBlockID(p.rootBlockID).SealedResult()
		if err != nil {
			// if the node was bootstrapped from a block after the spork root block, then the root
			// block's result will not be present.
			return nil, errors.Join(optimistic_sync.ErrBlockBeforeNodeHistory, err)
		}

		effectiveExecutors, err :=
			p.executionNodes.SelectExecutionNodes(availableExecutors, criteria.RequiredExecutors)
		if err != nil {
			return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", p.rootBlockID, err)
		}

		return &optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: rootBlockResult.ID(),
			ExecutionNodes:    effectiveExecutors,
		}, nil
	}

	resultID, executorIDs, err := p.findResultAndExecutors(blockID, criteria)
	if err != nil {
		if optimistic_sync.IsExecutionResultNotReadyError(err) {
			isBlockSealed, err := p.sealingStatusResolver.IsSealed(blockID)
			if err != nil {
				return nil, fmt.Errorf("failed to check if block sealed: %w", err)
			}

			// if the block is already sealed, then the criteria could not be satisfied.
			if isBlockSealed {
				return nil, optimistic_sync.NewCriteriaNotMetError(blockID)
			}
		}

		return nil, fmt.Errorf("failed to find result and executors for block ID %v: %w", blockID, err)
	}

	executors := availableExecutors.Filter(filter.HasNodeID[flow.Identity](executorIDs...))
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

// findResultAndExecutors returns a query response for a given block ID.
// The result must match the provided criteria and have at least one acceptable executor. If multiple
// results are found, then the result with the most executors is returned.
//
// Expected errors during normal operations:
//   - [optimistic_sync.ErrExecutionResultNotReady]: If criteria cannot be satisfied at the moment.
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
		// execution receipts must exist. if there are no receipts,
		// it indicates a bug in the access node syncing/notifying mechanism
		return flow.ZeroID, nil, err
	}

	// find all results that match the criteria and have at least one acceptable executor
	matchingResults := make([]resultWithReceipts, 0)
	for executionResultID, executionReceiptList := range allReceiptsForBlock.GroupByResultID() {
		result := &executionReceiptList[0].ExecutionResult
		executorToReceiptsMap := executionReceiptList.GroupByExecutorID()

		if !p.isExecutorGroupMeetingCriteria(result, executorToReceiptsMap, criteria) {
			// skip groups that don't meet criteria
			continue
		}

		matchingResults = append(matchingResults, resultWithReceipts{
			id:       executionResultID,
			receipts: executionReceiptList,
		})
	}

	if len(matchingResults) == 0 {
		return flow.ZeroID, nil, optimistic_sync.NewExecutionResultNotReadyError("no receipts that match criteria found")
	}

	// sort matchingResults by the number of execution nodes in descending order
	sort.Slice(matchingResults, func(i, j int) bool {
		return len(matchingResults[i].receipts) > len(matchingResults[j].receipts)
	})

	executorIDs := getExecutorIDs(matchingResults[0].receipts)
	return matchingResults[0].id, executorIDs, nil
}

// isExecutorGroupMeetingCriteria checks if an executor group meets the specified criteria for execution receipts matching.
func (p *Provider) isExecutorGroupMeetingCriteria(
	executionResult *flow.ExecutionResult,
	executorToExecutionReceipts flow.ExecutionReceiptGroupedList,
	criteria optimistic_sync.Criteria,
) bool {
	// check if there are enough executors
	if uint(len(executorToExecutionReceipts)) < criteria.AgreeingExecutorsCount {
		return false
	}

	// First, ensure the execution result is in the same execution fork, if any.
	// This avoids returning other errors (like required executor missing) for a fork
	// that we wouldn't consider anyway.
	if criteria.ParentExecutionResultID != flow.ZeroID &&
		executionResult.PreviousResultID != criteria.ParentExecutionResultID {
		return false
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
			return false
		}
	}

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
