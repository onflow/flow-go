package pipeline

import (
	"fmt"

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
	// ResultInFork is an optional ExecutionResult that must exist in the fork. Used to ensure consistency between requests
	ResultInFork flow.Identifier
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
	//   - ErrNoENsFoundForExecutionResult - returned when no execution nodes were found that produced
	//     the requested execution result and matches all operator's criteria.
	//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - All other errors are potential indicators of bugs or corrupted internal state
	ExecutionResultQuery(blockID flow.Identifier, criteria Criteria) (*Query, error)
}

// ErrNoENsFoundForExecutionResult is returned when no execution nodes were found that produced
// the requested execution result and matches all operator's criteria.
var ErrNoENsFoundForExecutionResult = fmt.Errorf("no execution nodes found for execution result")

var _ ExecutionResultQueryProvider = (*ExecutionResultQueryProviderImpl)(nil)

// ExecutionResultQueryProviderImpl is a container for elements required to retrieve
// execution results and execution node identities for a given block ID based on specified criteria.
type ExecutionResultQueryProviderImpl struct {
	log zerolog.Logger

	executionReceipts storage.ExecutionReceipts
	state             protocol.State

	preferredENIdentifiers flow.IdentifierList
	requiredENIdentifiers  flow.IdentifierList
	agreeingExecutors      uint

	rootBlockID     flow.Identifier
	rootBlockResult *flow.ExecutionResult
}

// NewExecutionResultQueryProviderImpl creates and returns a new instance of
// ExecutionResultQueryProviderImpl.
//
// Parameters:
//   - log: The logger to use for logging.
//   - state: The protocol state used for retrieving block information.
//   - executionReceipts: A storage.ExecutionReceipts object that contains the execution receipts
//     for blocks.
//   - preferredENIdentifiers: A flow.IdentifierList of preferred execution node identifiers that
//     are prioritized during selection.
//   - requiredENIdentifiers: A flow.IdentifierList of required execution node identifiers that are
//     always considered if available.
//   - agreeingExecutors: The minimum number of receipts including the same ExecutionResult.
//
// No errors are expected during normal operations
func NewExecutionResultQueryProviderImpl(
	log zerolog.Logger,
	state protocol.State,
	executionReceipts storage.ExecutionReceipts,
	preferredENIdentifiers flow.IdentifierList,
	requiredENIdentifiers flow.IdentifierList,
	agreeingExecutors uint,
) (*ExecutionResultQueryProviderImpl, error) {
	// Root block ID and result should not change and could be cached.
	rootBlock := state.Params().FinalizedRoot()
	rootBlockID := rootBlock.ID()
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
		agreeingExecutors:      agreeingExecutors,
		rootBlockID:            rootBlockID,
		rootBlockResult:        rootBlockResult,
	}, nil
}

// ExecutionResultQuery retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria.
//
// Expected errors during normal operations:
//   - ErrNoENsFoundForExecutionResult - returned when no execution nodes were found that produced
//     the requested execution result and matches all operator's criteria
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
//   - All other errors are potential indicators of bugs or corrupted internal state
func (e *ExecutionResultQueryProviderImpl) ExecutionResultQuery(blockID flow.Identifier, criteria Criteria) (*Query, error) {
	// if caller criteria are not set, the operator criteria should be used
	e.mergeCriteria(&criteria)

	// Check if the block ID is of the root block. If it is, then don't look for execution receipts since they
	// will not be present for the root block.
	if e.rootBlockID == blockID {
		return e.handleRootBlock(criteria)
	}

	execResult, executorIDs, err := e.findResultAndExecutors(blockID, criteria)
	if err != nil {
		return nil, err
	}

	subsetENs, err := e.chooseExecutionNodes(criteria.RequiredExecutors, executorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to choose execution nodes for block ID %v: %w", blockID, err)
	}

	if len(subsetENs) == 0 {
		return nil, ErrNoENsFoundForExecutionResult
	}

	return &Query{
		ExecutionResult: execResult,
		ExecutionNodes:  subsetENs,
	}, nil
}

// mergeCriteria merges criteria with base overrides. Fields from 'criteria' take precedence when set.
func (e *ExecutionResultQueryProviderImpl) mergeCriteria(criteria *Criteria) {
	if criteria.AgreeingExecutors == 0 {
		criteria.AgreeingExecutors = e.agreeingExecutors
	}

	if len(criteria.RequiredExecutors) == 0 {
		criteria.RequiredExecutors = e.requiredENIdentifiers
	}
}

// handleRootBlock handles execution result queries for the root block.
// Since root blocks don't have execution receipts, it returns all execution nodes
// from the identity table filtered by the specified criteria and nil execution result.
//
// No errors are expected during normal operations
func (e *ExecutionResultQueryProviderImpl) handleRootBlock(criteria Criteria) (*Query, error) {
	executorIdentities, err := e.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve execution IDs for root block: %w", err)
	}

	executorIDs := executorIdentities.NodeIDs()
	subsetENs, err := e.chooseExecutionNodes(criteria.RequiredExecutors, executorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to choose execution nodes for root block ID %v: %w", e.rootBlockID, err)
	}

	return &Query{
		ExecutionResult: e.rootBlockResult,
		ExecutionNodes:  subsetENs,
	}, nil
}

// findResultAndExecutors retrieves execution results and receipts for a given block ID
// that match the specified criteria. It groups receipts by result ID and selects
// the result with the most agreeing receipts or the specific result if specified in criteria.
//
// Expected errors during normal operations:
//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
//   - All other errors are potential indicators of bugs or corrupted internal state
func (e *ExecutionResultQueryProviderImpl) findResultAndExecutors(
	blockID flow.Identifier,
	criteria Criteria,
) (*flow.ExecutionResult, flow.IdentifierList, error) {
	// look up the receipt's storage with the block ID
	// Note: this will return an empty slice with no error if no receipts are found.
	allReceipts, err := e.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	executionReceiptsGroupedList := allReceipts.GroupByResultID()

	matchedExecutorsCnt := uint(0)
	var matchedResultID flow.Identifier

	for resultID, executionReceiptList := range executionReceiptsGroupedList {
		executorGroup := executionReceiptList.GroupByExecutorID()
		if !isReceptsMatchingCriteria(executorGroup, criteria) {
			continue
		}

		currentMatchedExecutorsCnt := uint(len(executorGroup))
		if currentMatchedExecutorsCnt > matchedExecutorsCnt {
			matchedExecutorsCnt = currentMatchedExecutorsCnt
			matchedResultID = resultID
		}
	}

	// if less than agreeing executors have been received, then return an error
	if matchedExecutorsCnt < criteria.AgreeingExecutors {
		return nil, nil, backend.NewInsufficientExecutionReceipts(blockID, int(matchedExecutorsCnt))
	}

	matchedReceipts := executionReceiptsGroupedList.GetGroup(matchedResultID)

	// all matched receipts are for the ExecutionResult.
	result := matchedReceipts[0].ExecutionResult
	executorIDs := getExecutorIDs(matchedReceipts)

	return &result, executorIDs, nil
}

// isReceptsMatchingCriteria checks if an executor group meets the specified criteria for execution receipts matching.
func isReceptsMatchingCriteria(executorGroup flow.ExecutionReceiptGroupedList, criteria Criteria) bool {
	currentMatchedExecutorsCnt := uint(len(executorGroup))

	if currentMatchedExecutorsCnt < criteria.AgreeingExecutors {
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
	// the current result's fork includes the requested result.
	// https://github.com/onflow/flow-go/issues/7587

	return true
}

// getExecutorIDs extracts unique executor node IDs from a list of execution receipts.
// It groups receipts by executor ID and returns all unique executor identifiers.
func getExecutorIDs(recepts flow.ExecutionReceiptList) flow.IdentifierList {
	receiptGroupedByExecutorID := recepts.GroupByExecutorID()

	// collect all unique execution node ids from the receipts
	executorIDs := make(flow.IdentifierList, 0, len(receiptGroupedByExecutorID))
	for executorID := range receiptGroupedByExecutorID {
		executorIDs = append(executorIDs, executorID)
	}

	return executorIDs
}

// chooseExecutionNodes finds the subset of execution nodes defined in the identity table by first
// choosing the preferred execution nodes which have executed the transaction. If no such preferred
// execution nodes are found, then the required execution nodes defined in the identity table are returned.
// If neither preferred nor required nodes are defined, then all execution nodes matching the executor IDs are returned.
//
// For example, if execution nodes in the identity table are {1,2,3,4}, preferred ENs are defined as {2,3,4},
// and the executor IDs are {1,2,3}, then {2, 3} is returned as the chosen subset of ENs.
//
// No errors are expected during normal operations
func (e *ExecutionResultQueryProviderImpl) chooseExecutionNodes(
	requiredExecutors flow.IdentifierList,
	executorIDs flow.IdentifierList,
) (flow.IdentitySkeletonList, error) {
	allENs, err := e.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve all execution IDs: %w", err)
	}

	if len(e.preferredENIdentifiers) > 0 {
		chosenIDs := e.chooseFromPreferredENIDs(allENs, executorIDs)
		return chosenIDs.ToSkeleton(), nil
	}

	// if no preferred EN ID is found, then choose from the required EN IDs
	// TODO: the order here means that if the operator has configured any preferred ENs,
	// the user's required EN check will be entirely skipped.
	// This will be addressed in https://github.com/onflow/flow-go/issues/7588
	if len(requiredExecutors) > 0 {
		// choose required ENs which have executed the transaction
		chosenIDs := allENs.Filter(filter.And(
			filter.HasNodeID[flow.Identity](requiredExecutors...),
			filter.HasNodeID[flow.Identity](executorIDs...),
		))
		if len(chosenIDs) > 0 {
			return chosenIDs.ToSkeleton(), nil
		}
		// if no such ENs are found, then just choose all required ENs
		chosenIDs = allENs.Filter(filter.HasNodeID[flow.Identity](requiredExecutors...))
		return chosenIDs.ToSkeleton(), nil
	}

	// if no preferred or required ENs have been specified, then return all executor IDs i.e., no preference at all
	return allENs.Filter(filter.HasNodeID[flow.Identity](executorIDs...)).ToSkeleton(), nil
}

// chooseFromPreferredENIDs finds the subset of execution nodes if preferred execution nodes are defined.
// If preferredENIdentifiers are set and there are less than maxNodesCnt nodes selected, then the list is padded up to
// maxNodesCnt nodes using the following order:
// 1. Use any EN with a receipt.
// 2. Use any preferred node not already selected.
// 3. Use any EN not already selected.
func (e *ExecutionResultQueryProviderImpl) chooseFromPreferredENIDs(
	allENs flow.IdentityList,
	executorIDs flow.IdentifierList,
) flow.IdentityList {
	var chosenIDs flow.IdentityList

	// filter for both preferred and executor IDs
	chosenIDs = allENs.Filter(filter.And(
		filter.HasNodeID[flow.Identity](e.preferredENIdentifiers...),
		filter.HasNodeID[flow.Identity](executorIDs...),
	))

	if len(chosenIDs) >= maxNodesCnt {
		return chosenIDs
	}

	// function to add nodes to chosenIDs if they are not already included
	addIfNotExists := func(candidates flow.IdentityList) {
		for _, en := range candidates {
			_, exists := chosenIDs.ByNodeID(en.NodeID)
			if !exists {
				chosenIDs = append(chosenIDs, en)
				if len(chosenIDs) >= maxNodesCnt {
					return
				}
			}
		}
	}

	// add any EN with a receipt
	receiptENs := allENs.Filter(filter.HasNodeID[flow.Identity](executorIDs...))
	addIfNotExists(receiptENs)
	if len(chosenIDs) >= maxNodesCnt {
		return chosenIDs
	}

	// add any preferred node not already selected
	preferredENs := allENs.Filter(filter.HasNodeID[flow.Identity](e.preferredENIdentifiers...))
	addIfNotExists(preferredENs)
	if len(chosenIDs) >= maxNodesCnt {
		return chosenIDs
	}

	// add any EN not already selected
	addIfNotExists(allENs)

	return chosenIDs
}
