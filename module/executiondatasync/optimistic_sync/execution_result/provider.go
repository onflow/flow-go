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
	state             protocol.State

	executionNodeSelector *ExecutionNodeSelector

	rootBlockID     flow.Identifier
	rootBlockResult *flow.ExecutionResult

	operatorCriteria optimistic_sync.Criteria
}

var _ optimistic_sync.ExecutionResultInfoProvider = (*Provider)(nil)

// NewExecutionResultInfoProvider creates and returns a new instance of
// Provider.
//
// No errors are expected during normal operations
func NewExecutionResultInfoProvider(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	executionReceipts storage.ExecutionReceipts,
	executionNodeSelector *ExecutionNodeSelector,
	operatorCriteria optimistic_sync.Criteria,
) (*Provider, error) {
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

	return &Provider{
		log:                   log.With().Str("module", "execution_result_info_provider").Logger(),
		executionReceipts:     executionReceipts,
		state:                 state,
		executionNodeSelector: executionNodeSelector,
		rootBlockID:           rootBlockID,
		rootBlockResult:       rootBlockResult,
		operatorCriteria:      optimistic_sync.DefaultCriteria.OverrideWith(operatorCriteria),
	}, nil
}

// ExecutionResultInfo retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria.
//
// Expected errors during normal operations:
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
func (p *Provider) ExecutionResultInfo(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (*optimistic_sync.ExecutionResultInfo, error) {
	allExecutors, err := p.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve execution IDs for root block: %w", err)
	}

	// if the block ID is the root block, then use the root ExecutionResult and skip the receipt
	// check since there will not be any.
	if p.rootBlockID == blockID {
		chosenExecutionNodes, err := p.executionNodeSelector.SelectExecutionNodes(
			allExecutors,
			criteria.RequiredExecutors,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", p.rootBlockID, err)
		}

		return &optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: p.rootBlockResult.ID(),
			ExecutionNodes:    chosenExecutionNodes,
		}, nil
	}

	resultID, executorIDsForBlock, err := p.findExecutionResultAndExecutors(blockID, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to find result and executors for block ID %v: %w", blockID, err)
	}

	executorsForBlock := allExecutors.Filter(filter.HasNodeID[flow.Identity](executorIDsForBlock...))
	filteredExecutors, err := p.executionNodeSelector.SelectExecutionNodes(executorsForBlock, criteria.RequiredExecutors)
	if err != nil {
		return nil, fmt.Errorf("failed to choose execution nodes for block ID %v: %w", blockID, err)
	}

	if len(filteredExecutors) == 0 {
		// this is unexpected, and probably indicates there is a bug.
		// There are only three ways that SelectExecutionNodes can return an empty list:
		//   1. there are no executors for the result
		//   2. none of the user's required executors are in the executor list
		//   3. none of the operator's required executors are in the executor list
		// None of these are possible since there must be at least one AgreeingExecutorsCount. If the
		// criteria is met, then there must be at least one acceptable executor. If this is not true,
		// then the criteria check must fail.
		return nil, fmt.Errorf("no execution nodes found for result %v (blockID: %v): %w",
			resultID, blockID, err)
	}

	return &optimistic_sync.ExecutionResultInfo{
		ExecutionResultID: resultID,
		ExecutionNodes:    filteredExecutors,
	}, nil
}

// findExecutionResultAndExecutors returns a query response for a given block ID.
// The result must match the provided criteria and have at least one acceptable executor. If multiple
// results are found, then the result with the most executors is returned.
//
// Expected errors during normal operations:
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
func (p *Provider) findExecutionResultAndExecutors(
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (flow.Identifier, flow.IdentifierList, error) {
	type resultWithReceipts struct {
		id       flow.Identifier
		receipts flow.ExecutionReceiptList
	}

	criteria = p.operatorCriteria.OverrideWith(criteria)

	// Note: this will return an empty slice with no error if no receipts are found.
	allReceiptsForBlock, err := p.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, nil,
			fmt.Errorf("failed to retreive execution receipts for block ID %v: %w",
				blockID, err)
	}

	// find all results that match the criteria and have at least one acceptable executor
	results := make([]resultWithReceipts, 0)
	for executionResultID, executionReceiptList := range allReceiptsForBlock.GroupByResultID() {
		result := &executionReceiptList[0].ExecutionResult
		receipts := executionReceiptList.GroupByExecutorID()

		if p.isExecutorGroupMeetingCriteria(result, receipts, criteria) {
			results = append(results, resultWithReceipts{
				id:       executionResultID,
				receipts: executionReceiptList,
			})
		}
	}

	if len(results) == 0 {
		return flow.ZeroID, nil, common.NewInsufficientExecutionReceipts(blockID, 0)
	}

	// sort results by the number of execution nodes in descending order
	sort.Slice(results, func(i, j int) bool {
		return len(results[i].receipts) > len(results[j].receipts)
	})

	executorIDs := getExecutorIDs(results[0].receipts)
	return results[0].id, executorIDs, nil
}

// isExecutorGroupMeetingCriteria checks if an executor group meets the specified criteria for execution receipts matching.
func (p *Provider) isExecutorGroupMeetingCriteria(
	executionResult *flow.ExecutionResult,
	executorToExecutionReceipts flow.ExecutionReceiptGroupedList,
	criteria optimistic_sync.Criteria,
) bool {
	if uint(len(executorToExecutionReceipts)) < criteria.AgreeingExecutorsCount {
		return false
	}

	// make sure one of the required executors is in the group
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

	// make sure the execution result is in the same execution fork, if any
	if criteria.ParentExecutionResultID != flow.ZeroID &&
		executionResult.PreviousResultID != criteria.ParentExecutionResultID {
		return false
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
