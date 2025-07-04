package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// minExecutionNodesCnt is the minimum number of execution nodes expected to have sent the execution receipt for a block
	minExecutionNodesCnt = 2
	// maxAttemptsForExecutionReceipt is the maximum number of attempts to find execution receipts for a given block ID
	maxAttemptsForExecutionReceipt = 3
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
	//   - InsufficientExecutionReceipts - If no such execution node is found.
	//   - All other errors are potential indicators of bugs or corrupted internal state
	ExecutionResultQuery(ctx context.Context, blockID flow.Identifier, criteria Criteria) (*Query, error)
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

	rootBlockID flow.Identifier
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
func NewExecutionResultQueryProviderImpl(
	log zerolog.Logger,
	state protocol.State,
	executionReceipts storage.ExecutionReceipts,
	preferredENIdentifiers flow.IdentifierList,
	requiredENIdentifiers flow.IdentifierList,
	agreeingExecutors uint,
) *ExecutionResultQueryProviderImpl {
	return &ExecutionResultQueryProviderImpl{
		log:                    log.With().Str("module", "execution_result_query").Logger(),
		executionReceipts:      executionReceipts,
		state:                  state,
		preferredENIdentifiers: preferredENIdentifiers,
		requiredENIdentifiers:  requiredENIdentifiers,
		agreeingExecutors:      agreeingExecutors,
		rootBlockID:            flow.ZeroID,
	}
}

// ExecutionResultQuery retrieves execution results and associated execution nodes for a given block ID
// based on the provided criteria. It implements retry logic to handle cases where execution receipts
// may not be immediately available and applies node selection based on preferences and requirements.
//
// Expected errors during normal operations:
//   - ErrNoENsFoundForExecutionResult - returned when no execution nodes were found that produced
//     the requested execution result and matches all operator's criteria
//   - InsufficientExecutionReceipts - If no such execution node is found.
//   - All other errors are potential indicators of bugs or corrupted internal state
func (e *ExecutionResultQueryProviderImpl) ExecutionResultQuery(ctx context.Context, blockID flow.Identifier, criteria Criteria) (*Query, error) {
	// if caller criteria does not set, the operators criteria should be used
	if criteria.AgreeingExecutors == 0 {
		criteria.AgreeingExecutors = e.agreeingExecutors
	}

	if len(criteria.RequiredExecutors) == 0 {
		criteria.RequiredExecutors = e.requiredENIdentifiers
	}

	// Root block ID should not change and could be cached.
	if e.rootBlockID == flow.ZeroID {
		rootBlock := e.state.Params().FinalizedRoot()
		e.rootBlockID = rootBlock.ID()
	}

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	if e.rootBlockID == blockID {
		return e.handleRootBlock(criteria)
	}

	var (
		executorIDs flow.IdentifierList
		execResult  *flow.ExecutionResult
	)

	// try to find at least minExecutionNodesCnt execution node ids from the execution receipts for the given blockID
	for attempt := 0; attempt < maxAttemptsForExecutionReceipt; attempt++ {
		result, receipts, err := e.findResultAndReceipts(blockID, criteria)
		if err != nil {
			return nil, err
		}

		executorIDs = getExecutorIDs(receipts)
		if len(executorIDs) >= minExecutionNodesCnt {
			execResult = result
			break
		}

		// log the attempt
		e.log.Debug().Int("attempt", attempt).Int("max_attempt", maxAttemptsForExecutionReceipt).
			Int("execution_receipts_found", len(executorIDs)).
			Str("block_id", blockID.String()).
			Msg("insufficient execution receipts")

		// if one or less execution receipts may have been received then re-query
		// in the hope that more might have been received by now
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond << time.Duration(attempt)):
			// retry after an exponential backoff
		}
	}

	// if less than minExecutionNodesCnt execution receipts have been received then return an error
	if len(executorIDs) < minExecutionNodesCnt {
		return nil, fmt.Errorf("failed to retreive minimum execution nodes for block ID %v", blockID)
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

// handleRootBlock handles execution result queries for the root block.
// Since root blocks don't have execution receipts, it returns all execution nodes
// from the identity table filtered by the specified criteria and nil execution result.
// Expected errors during normal operations:
//   - InsufficientExecutionReceipts - If no such execution node is found.
//   - ErrNoMatchingExecutionResult - when no execution result matching the criteria was found
//   - All other errors are potential indicators of bugs or corrupted internal state
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

	if len(subsetENs) == 0 {
		return nil, ErrNoENsFoundForExecutionResult
	}

	return &Query{
		ExecutionResult: nil,
		ExecutionNodes:  subsetENs,
	}, nil
}

// findResultAndReceipts retrieves execution results and receipts for a given block ID
// that match the specified criteria. It groups receipts by result ID and selects
// the result with the most agreeing receipts or the specific result if specified in criteria.
//
// No errors are expected during normal operations
func (e *ExecutionResultQueryProviderImpl) findResultAndReceipts(
	blockID flow.Identifier,
	criteria Criteria,
) (*flow.ExecutionResult, flow.ExecutionReceiptList, error) {
	// lookup the receipt's storage with the block ID
	allReceipts, err := e.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	executionReceiptsGroupedList := allReceipts.GroupByResultID()
	if executionReceiptsGroupedList.NumberGroups() > 1 {
		identicalReceiptsStr := fmt.Sprintf("%v", flow.GetIDs(allReceipts))
		e.log.Error().
			Str("block_id", blockID.String()).
			Str("execution_receipts", identicalReceiptsStr).
			Msg("execution receipt mismatch")
	}

	var matchedReceipts flow.ExecutionReceiptList

	if criteria.ResultInFork != flow.ZeroID {
		matchedReceipts = executionReceiptsGroupedList.GetGroup(criteria.ResultInFork)
	} else {
		// maximum number of matching receipts found so far for any execution result id
		maxMatchedReceiptCnt := 0
		var maxMatchedResultID flow.Identifier
		// find the largest list of receipts which have the same result ID
		for resultID, executionReceiptList := range executionReceiptsGroupedList {
			currentMatchedReceiptCnt := len(executionReceiptList)
			if currentMatchedReceiptCnt > maxMatchedReceiptCnt {
				maxMatchedReceiptCnt = currentMatchedReceiptCnt
				maxMatchedResultID = resultID
			}
		}

		// pick the largest list of matching receipts
		matchedReceipts = executionReceiptsGroupedList.GetGroup(maxMatchedResultID)
	}

	if len(matchedReceipts) < int(criteria.AgreeingExecutors) {
		return nil, nil, fmt.Errorf("not enough agreeing receipts for execution result")
	}

	// as all matched recepts are for the same block they should have same ExecutionResult, that is why return first result in a list
	result := matchedReceipts[0].ExecutionResult

	return &result, matchedReceipts, nil
}

// getExecutorIDs extracts unique executor node IDs from a list of execution receipts.
// It groups receipts by executor ID and returns all unique executor identifiers.
func getExecutorIDs(recepts flow.ExecutionReceiptList) flow.IdentifierList {
	receiptGroupedByExecutorID := recepts.GroupByExecutorID()

	// collect all unique execution node ids from the receipts
	executorIDs := flow.IdentifierList{}
	for executorID := range receiptGroupedByExecutorID {
		executorIDs = append(executorIDs, executorID)
	}

	return executorIDs
}

// chooseExecutionNodes finds the subset of execution nodes defined in the identity table by first
// choosing the preferred execution nodes which have executed the transaction. If no such preferred
// execution nodes are found, then the required execution nodes defined in the identity table are returned.
// If neither preferred nor required nodes are defined, then all execution node matching the executor IDs are returned.
//
// For example: If execution nodes in identity table are {1,2,3,4}, preferred ENs are defined as {2,3,4}
// and the executor IDs is {1,2,3}, then {2, 3} is returned as the chosen subset of ENs.
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
// If preferredENIdentifiers is set and there are less than maxNodesCnt nodes selected, then the list is padded up to
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
