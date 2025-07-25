package pipeline

import (
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// maxNodesCnt is the maximum number of nodes that will be contacted to complete an API request.
	maxNodesCnt = 3
)

// Criteria defines the filtering criteria for execution result queries.
// It specifies requirements for execution result selection including the number
// of agreeing executors, required executor nodes, and consistency constraints.
type Criteria struct {
	// AgreeingExecutors is the number of receipts including the same ExecutionResult
	AgreeingExecutors uint
	// RequiredExecutors is the list of EN node IDs, one of which must have produced the result
	RequiredExecutors flow.IdentifierList
}

// Merge merges the `incoming` criteria into the current criteria, returning a new Criteria object.
// Fields from `incoming` take precedence when set.
func (c Criteria) Merge(incoming Criteria) Criteria {
	merged := c

	if incoming.AgreeingExecutors > 0 {
		merged.AgreeingExecutors = incoming.AgreeingExecutors
	}

	if len(incoming.RequiredExecutors) > 0 {
		merged.RequiredExecutors = incoming.RequiredExecutors
	}

	return merged
}

// DefaultCriteria is the system default criteria for execution result queries.
var DefaultCriteria = Criteria{
	AgreeingExecutors: 2,
}

// Query contains the result of an execution result query.
// It includes both the execution result and the execution nodes that produced it.
type Query struct {
	// ExecutionResult is the execution result for the queried block
	ExecutionResult *flow.ExecutionResult
	// ExecutionNodes is the list of execution node identities that produced the result
	ExecutionNodes flow.IdentitySkeletonList
}

// ExecutionResultQueryProvider provides execution results and execution nodes based on criteria.
// It allows querying for execution results by block ID with specific filtering criteria
// to ensure consistency and reliability of execution results.
type ExecutionResultQueryProvider interface {
	// ExecutionResultQuery retrieves execution results and associated execution nodes for a given block ID
	// based on the provided criteria. It returns a Query containing the execution result and
	// the execution nodes that produced it.
	//
	// Expected errors during normal operations:
	//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - All other errors are potential indicators of bugs or corrupted internal state
	ExecutionResultQuery(blockID flow.Identifier, criteria Criteria) (*Query, error)
}

var _ ExecutionResultQueryProvider = (*ExecutionResultQueryProviderImpl)(nil)

// ExecutionResultQueryProviderImpl is a container for elements required to retrieve
// execution results and execution node identities for a given block ID based on specified criteria.
type ExecutionResultQueryProviderImpl struct {
	log zerolog.Logger

	executionReceipts storage.ExecutionReceipts
	state             protocol.State

	preferredENIdentifiers flow.IdentifierList
	requiredENIdentifiers  flow.IdentifierList

	rootBlockID     flow.Identifier
	rootBlockResult *flow.ExecutionResult

	baseCriteria Criteria
}

// NewExecutionResultQueryProviderImpl creates and returns a new instance of
// ExecutionResultQueryProviderImpl.
//
// No errors are expected during normal operations
func NewExecutionResultQueryProviderImpl(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	executionReceipts storage.ExecutionReceipts,
	preferredENIdentifiers flow.IdentifierList,
	requiredENIdentifiers flow.IdentifierList,
	operatorCriteria Criteria,
) (*ExecutionResultQueryProviderImpl, error) {
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

	return &ExecutionResultQueryProviderImpl{
		log:                    log.With().Str("module", "execution_result_query").Logger(),
		executionReceipts:      executionReceipts,
		state:                  state,
		preferredENIdentifiers: preferredENIdentifiers,
		requiredENIdentifiers:  requiredENIdentifiers,
		rootBlockID:            rootBlockID,
		rootBlockResult:        rootBlockResult,
		baseCriteria:           DefaultCriteria.Merge(operatorCriteria),
	}, nil
}

// ExecutionResultQuery retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria.
//
// Expected errors during normal operations:
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
//   - All other errors are potential indicators of bugs or corrupted internal state
func (e *ExecutionResultQueryProviderImpl) ExecutionResultQuery(blockID flow.Identifier, criteria Criteria) (*Query, error) {
	executorIdentities, err := e.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve execution IDs for root block: %w", err)
	}

	// if the block ID is the root block, then use the root ExecutionResult and skip the receipt
	// check since there will not be any.
	if e.rootBlockID == blockID {
		subsetENs, err := e.chooseExecutionNodes(executorIdentities, criteria.RequiredExecutors)
		if err != nil {
			return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", e.rootBlockID, err)
		}

		return &Query{
			ExecutionResult: e.rootBlockResult,
			ExecutionNodes:  subsetENs,
		}, nil
	}

	result, executorIDs, err := e.findResultAndExecutors(blockID, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to find result and executors for block ID %v: %w", blockID, err)
	}

	executors := executorIdentities.Filter(filter.HasNodeID[flow.Identity](executorIDs...))
	subsetENs, err := e.chooseExecutionNodes(executors, criteria.RequiredExecutors)
	if err != nil {
		return nil, fmt.Errorf("failed to choose execution nodes for block ID %v: %w", blockID, err)
	}

	if len(subsetENs) == 0 {
		// this is unexpected, and probably indicates there is a bug.
		// There are only three ways that chooseExecutionNodes can return an empty list:
		//   1. there are no executors for the result
		//   2. none of the user's required executors are in the executor list
		//   3. none of the operator's required executors are in the executor list
		// None of these are possible since there must be at least one AgreeingExecutors. If the
		// criteria is met, then there must be at least one acceptable executor. If this is not true,
		// then the criteria check must fail.
		return nil, fmt.Errorf("no execution nodes found for result %v (blockID: %v): %w", result.ID(), blockID, err)
	}

	return &Query{
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
//   - All other errors are potential indicators of bugs or corrupted internal state
func (e *ExecutionResultQueryProviderImpl) findResultAndExecutors(
	blockID flow.Identifier,
	criteria Criteria,
) (*flow.ExecutionResult, flow.IdentifierList, error) {
	type result struct {
		result   *flow.ExecutionResult
		receipts flow.ExecutionReceiptList
	}

	criteria = e.baseCriteria.Merge(criteria)

	// Note: this will return an empty slice with no error if no receipts are found.
	allReceipts, err := e.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	// find all results that match the criteria and have at least one acceptable executor
	results := make([]result, 0)
	for _, executionReceiptList := range allReceipts.GroupByResultID() {
		executorGroup := executionReceiptList.GroupByExecutorID()
		if checkCriteria(executorGroup, criteria) {
			results = append(results, result{
				result:   &executionReceiptList[0].ExecutionResult,
				receipts: executionReceiptList,
			})
		}
	}

	if len(results) == 0 {
		return nil, nil, backend.NewInsufficientExecutionReceipts(blockID, 0)
	}

	// sort results by the number of execution nodes in descending order
	sort.Slice(results, func(i, j int) bool {
		return len(results[i].receipts) > len(results[j].receipts)
	})

	executorIDs := getExecutorIDs(results[0].receipts)
	return results[0].result, executorIDs, nil
}

// checkCriteria checks if an executor group meets the specified criteria for execution receipts matching.
func checkCriteria(executorGroup flow.ExecutionReceiptGroupedList, criteria Criteria) bool {
	if uint(len(executorGroup)) < criteria.AgreeingExecutors {
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
func getExecutorIDs(recepts flow.ExecutionReceiptList) flow.IdentifierList {
	receiptGroupedByExecutorID := recepts.GroupByExecutorID()

	executorIDs := make(flow.IdentifierList, 0, len(receiptGroupedByExecutorID))
	for executorID := range receiptGroupedByExecutorID {
		executorIDs = append(executorIDs, executorID)
	}

	return executorIDs
}

// chooseExecutionNodes finds the subset of execution nodes defined in the identity table that matches
// the provided executor IDs and executor criteria.
//
// The following precedence is used to determine the subset of execution nodes:
//
//  1. If the user's RequiredExecutors is not empty, only select executors from their list
//
//  2. If the operator's `requiredENIdentifiers` is set, only select executors from the required ENs list.
//     If the operator's `preferredENIdentifiers` is also set, then the preferred ENs are selected first.
//
//  3. If only the operator's `preferredENIdentifiers` is set, then select any preferred ENs that
//     have executed the result, and fall back to selecting any ENs that have executed the result.
//
//  4. If neither preferred nor required nodes are defined, then all execution nodes matching the
//     executor IDs are returned.
//
// No errors are expected during normal operations
func (e *ExecutionResultQueryProviderImpl) chooseExecutionNodes(
	executors flow.IdentityList,
	userRequiredExecutors flow.IdentifierList,
) (flow.IdentitySkeletonList, error) {
	var chosenIDs flow.IdentityList

	// first, check if the user's criteria included any required executors.
	// since the result is chosen based on the user's required executors, this should always return
	// at least one match.
	if len(userRequiredExecutors) > 0 {
		chosenIDs = executors.Filter(filter.And(filter.HasNodeID[flow.Identity](userRequiredExecutors...)))
		return chosenIDs.ToSkeleton(), nil
	}

	// if required ENs are set, only select executors from the required ENs list
	// similarly, if the user does not provide any required executors, then the operator's
	// `e.requiredENIdentifiers` are applied, so this should always return at least one match.
	if len(e.requiredENIdentifiers) > 0 {
		chosenIDs = e.chooseFromRequiredENIDs(executors)
		return chosenIDs.ToSkeleton(), nil
	}

	// if only preferred ENs are set, then select any preferred ENs that have executed the result,
	// and fall back to selecting any executors.
	if len(e.preferredENIdentifiers) > 0 {
		chosenIDs = executors.Filter(filter.And(filter.HasNodeID[flow.Identity](e.preferredENIdentifiers...)))
		if len(chosenIDs) >= maxNodesCnt {
			return chosenIDs.ToSkeleton(), nil
		}
	}

	// finally, add any remaining required executors
	chosenIDs = addIfNotExists(chosenIDs, executors)
	return chosenIDs.ToSkeleton(), nil
}

// chooseFromRequiredENIDs finds the subset the provided executors that match the required ENs.
// if `e.preferredENIdentifiers` is not empty, then any preferred ENs that have executed the result
// will be added to the subset.
// otherwise, any executor in the `e.requiredENIdentifiers` list will be returned.
func (e *ExecutionResultQueryProviderImpl) chooseFromRequiredENIDs(
	executors flow.IdentityList,
) flow.IdentityList {
	var chosenIDs flow.IdentityList

	// add any preferred ENs that have executed the result and return if there are enough nodes
	// if both preferred and required ENs are set, then preferred MUST be a subset of required
	if len(e.preferredENIdentifiers) > 0 {
		chosenIDs = executors.Filter(filter.And(filter.HasNodeID[flow.Identity](e.preferredENIdentifiers...)))
		if len(chosenIDs) >= maxNodesCnt {
			return chosenIDs
		}
	}

	// next, add any other required ENs that have executed the result
	executedRequired := executors.Filter(filter.And(filter.HasNodeID[flow.Identity](e.requiredENIdentifiers...)))
	chosenIDs = addIfNotExists(chosenIDs, executedRequired)

	return chosenIDs
}

// function to add nodes to chosenIDs if they are not already included
func addIfNotExists(chosenIDs, candidates flow.IdentityList) flow.IdentityList {
	for _, en := range candidates {
		if _, exists := chosenIDs.ByNodeID(en.NodeID); !exists {
			chosenIDs = append(chosenIDs, en)
			if len(chosenIDs) >= maxNodesCnt {
				break
			}
		}
	}

	return chosenIDs
}
