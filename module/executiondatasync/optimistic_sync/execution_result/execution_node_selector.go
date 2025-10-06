package execution_result

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

const (
	// defaultMaxNodesCnt is the maximum number of nodes that will be contacted to complete an API request.
	defaultMaxNodesCnt = 3
)

// ExecutionNodeSelector handles the selection of execution nodes based on preferences and requirements.
// It encapsulates the logic for choosing execution nodes based on operator preferences, operator requirements,
// and user requirements.
type ExecutionNodeSelector struct {
	// preferredENIdentifiers are the execution nodes that the operator prefers to use
	preferredENIdentifiers flow.IdentifierList
	// requiredENIdentifiers are the execution nodes that the operator requires to use
	requiredENIdentifiers flow.IdentifierList
	// maxNodesCnt is the maximum number of nodes to select
	maxNodesCnt int
}

// NewExecutionNodeSelector creates a new ExecutionNodeSelector with the provided configuration.
func NewExecutionNodeSelector(
	preferredENIdentifiers flow.IdentifierList,
	requiredENIdentifiers flow.IdentifierList,
) *ExecutionNodeSelector {
	return &ExecutionNodeSelector{
		preferredENIdentifiers: preferredENIdentifiers,
		requiredENIdentifiers:  requiredENIdentifiers,
		maxNodesCnt:            defaultMaxNodesCnt,
	}
}

// SelectExecutionNodes finds the subset of execution nodes defined in the identity table that matches
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
func (en *ExecutionNodeSelector) SelectExecutionNodes(
	executors flow.IdentityList,
	userRequiredExecutors flow.IdentifierList,
) (flow.IdentitySkeletonList, error) {
	var chosenIDs flow.IdentityList

	// first, check if the user's criteria included any required executors.
	// since the result is chosen based on the user's required executors, this should always return
	// at least one match.
	if len(userRequiredExecutors) > 0 {
		chosenIDs = executors.Filter(filter.HasNodeID[flow.Identity](userRequiredExecutors...))
		return chosenIDs.ToSkeleton(), nil
	}

	// if required ENs are set, only select executors from the required ENs list
	// similarly, if the user does not provide any required executors, then the operator's
	// `en.requiredENIdentifiers` are applied, so this should always return at least one match.
	if len(en.requiredENIdentifiers) > 0 {
		chosenIDs = en.selectFromRequiredENIDs(executors)
		return chosenIDs.ToSkeleton(), nil
	}

	// if only preferred ENs are set, then select any preferred ENs that have executed the result,
	// and fall back to selecting any executors.
	if len(en.preferredENIdentifiers) > 0 {
		chosenIDs = executors.Filter(filter.HasNodeID[flow.Identity](en.preferredENIdentifiers...))
		if len(chosenIDs) >= en.maxNodesCnt {
			return chosenIDs.ToSkeleton(), nil
		}
	}

	// finally, add any remaining required executors
	chosenIDs = en.mergeExecutionNodes(chosenIDs, executors)
	return chosenIDs.ToSkeleton(), nil
}

// selectFromRequiredENIDs finds the subset the provided executors that match the required ENs.
// if `e.preferredENIdentifiers` is not empty, then any preferred ENs that have executed the result
// will be added to the subset.
// otherwise, any executor in the `e.requiredENIdentifiers` list will be returned.
func (en *ExecutionNodeSelector) selectFromRequiredENIDs(
	executors flow.IdentityList,
) flow.IdentityList {
	var chosenIDs flow.IdentityList

	// add any preferred ENs that have executed the result and return if there are enough nodes
	// if both preferred and required ENs are set, then preferred MUST be a subset of required
	if len(en.preferredENIdentifiers) > 0 {
		chosenIDs = executors.Filter(filter.HasNodeID[flow.Identity](en.preferredENIdentifiers...))
		if len(chosenIDs) >= en.maxNodesCnt {
			return chosenIDs
		}
	}

	// next, add any other required ENs that have executed the result
	executedRequired := executors.Filter(filter.HasNodeID[flow.Identity](en.requiredENIdentifiers...))
	chosenIDs = en.mergeExecutionNodes(chosenIDs, executedRequired)

	return chosenIDs
}

// mergeExecutionNodes adds nodes to chosenIDs if they are not already included
func (en *ExecutionNodeSelector) mergeExecutionNodes(chosenIDs, candidates flow.IdentityList) flow.IdentityList {
	for _, candidateNode := range candidates {
		if _, exists := chosenIDs.ByNodeID(candidateNode.NodeID); !exists {
			chosenIDs = append(chosenIDs, candidateNode)
			if len(chosenIDs) >= en.maxNodesCnt {
				break
			}
		}
	}

	return chosenIDs
}
