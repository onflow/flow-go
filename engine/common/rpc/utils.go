package rpc

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

// minExecutionNodesCnt is the minimum number of execution nodes expected to have sent the execution receipt for a block
const minExecutionNodesCnt = 2

// maxAttemptsForExecutionReceipt is the maximum number of attempts to find execution receipts for a given block ID
const maxAttemptsForExecutionReceipt = 3

// MaxNodesCnt is the maximum number of nodes that will be contacted to complete an API request.
const MaxNodesCnt = 3

func IdentifierList(ids []string) (flow.IdentifierList, error) {
	idList := make(flow.IdentifierList, len(ids))
	for i, idStr := range ids {
		id, err := flow.HexStringToIdentifier(idStr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert node id string %s to Flow Identifier: %w", id, err)
		}
		idList[i] = id
	}
	return idList, nil
}

// ExecutionNodesForBlockID returns upto maxNodesCnt number of randomly chosen execution node identities
// which have executed the given block ID.
// If no such execution node is found, an InsufficientExecutionReceipts error is returned.
func ExecutionNodesForBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	state protocol.State,
	log zerolog.Logger,
	preferredENIdentifiers flow.IdentifierList,
	fixedENIdentifiers flow.IdentifierList,
) (flow.IdentitySkeletonList, error) {
	var (
		executorIDs flow.IdentifierList
		err         error
	)

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	rootBlock := state.Params().FinalizedRoot()

	if rootBlock.ID() == blockID {
		executorIdentities, err := state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
		if err != nil {
			return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
		}
		executorIDs = executorIdentities.NodeIDs()
	} else {
		// try to find at least minExecutionNodesCnt execution node ids from the execution receipts for the given blockID
		for attempt := 0; attempt < maxAttemptsForExecutionReceipt; attempt++ {
			executorIDs, err = findAllExecutionNodes(blockID, executionReceipts, log)
			if err != nil {
				return nil, err
			}

			if len(executorIDs) >= minExecutionNodesCnt {
				break
			}

			// log the attempt
			log.Debug().Int("attempt", attempt).Int("max_attempt", maxAttemptsForExecutionReceipt).
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

		receiptCnt := len(executorIDs)
		// if less than minExecutionNodesCnt execution receipts have been received so far, then return random ENs
		if receiptCnt < minExecutionNodesCnt {
			newExecutorIDs, err := state.AtBlockID(blockID).Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
			if err != nil {
				return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
			}
			executorIDs = newExecutorIDs.NodeIDs()
		}
	}

	// choose from the preferred or fixed execution nodes
	subsetENs, err := chooseExecutionNodes(state, executorIDs, preferredENIdentifiers, fixedENIdentifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	if len(subsetENs) == 0 {
		return nil, fmt.Errorf("no matching execution node found for block ID %v", blockID)
	}

	return subsetENs, nil
}

// findAllExecutionNodes find all the execution nodes ids from the execution receipts that have been received for the
// given blockID
func findAllExecutionNodes(
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	log zerolog.Logger,
) (flow.IdentifierList, error) {
	// lookup the receipt's storage with the block ID
	allReceipts, err := executionReceipts.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	executionResultMetaList := make(flow.ExecutionReceiptMetaList, 0, len(allReceipts))
	for _, r := range allReceipts {
		executionResultMetaList = append(executionResultMetaList, r.Meta())
	}
	executionResultGroupedMetaList := executionResultMetaList.GroupByResultID()

	// maximum number of matching receipts found so far for any execution result id
	maxMatchedReceiptCnt := 0
	// execution result id key for the highest number of matching receipts in the identicalReceipts map
	var maxMatchedReceiptResultID flow.Identifier

	// find the largest list of receipts which have the same result ID
	for resultID, executionReceiptList := range executionResultGroupedMetaList {
		currentMatchedReceiptCnt := executionReceiptList.Size()
		if currentMatchedReceiptCnt > maxMatchedReceiptCnt {
			maxMatchedReceiptCnt = currentMatchedReceiptCnt
			maxMatchedReceiptResultID = resultID
		}
	}

	// if there are more than one execution result for the same block ID, log as error
	if executionResultGroupedMetaList.NumberGroups() > 1 {
		identicalReceiptsStr := fmt.Sprintf("%v", flow.GetIDs(allReceipts))
		log.Error().
			Str("block_id", blockID.String()).
			Str("execution_receipts", identicalReceiptsStr).
			Msg("execution receipt mismatch")
	}

	// pick the largest list of matching receipts
	matchingReceiptMetaList := executionResultGroupedMetaList.GetGroup(maxMatchedReceiptResultID)

	metaReceiptGroupedByExecutorID := matchingReceiptMetaList.GroupByExecutorID()

	// collect all unique execution node ids from the receipts
	var executorIDs flow.IdentifierList
	for executorID := range metaReceiptGroupedByExecutorID {
		executorIDs = append(executorIDs, executorID)
	}

	return executorIDs, nil
}

// chooseExecutionNodes finds the subset of execution nodes defined in the identity table by first
// choosing the preferred execution nodes which have executed the transaction. If no such preferred
// execution nodes are found, then the fixed execution nodes defined in the identity table are returned
// If neither preferred nor fixed nodes are defined, then all execution node matching the executor IDs are returned.
// e.g. If execution nodes in identity table are {1,2,3,4}, preferred ENs are defined as {2,3,4}
// and the executor IDs is {1,2,3}, then {2, 3} is returned as the chosen subset of ENs
func chooseExecutionNodes(state protocol.State,
	executorIDs flow.IdentifierList,
	preferredENIdentifiers flow.IdentifierList,
	fixedENIdentifiers flow.IdentifierList,
) (flow.IdentitySkeletonList, error) {
	allENs, err := state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve all execution IDs: %w", err)
	}

	// choose from preferred EN IDs
	if len(preferredENIdentifiers) > 0 {
		chosenIDs := ChooseFromPreferredENIDs(allENs, executorIDs, preferredENIdentifiers)
		return chosenIDs.ToSkeleton(), nil
	}

	// if no preferred EN ID is found, then choose from the fixed EN IDs
	if len(fixedENIdentifiers) > 0 {
		// choose fixed ENs which have executed the transaction
		chosenIDs := allENs.Filter(filter.And(
			filter.HasNodeID[flow.Identity](fixedENIdentifiers...),
			filter.HasNodeID[flow.Identity](executorIDs...),
		))
		if len(chosenIDs) > 0 {
			return chosenIDs.ToSkeleton(), nil
		}
		// if no such ENs are found, then just choose all fixed ENs
		chosenIDs = allENs.Filter(filter.HasNodeID[flow.Identity](fixedENIdentifiers...))
		return chosenIDs.ToSkeleton(), nil
	}

	// if no preferred or fixed ENs have been specified, then return all executor IDs i.e., no preference at all
	return allENs.Filter(filter.HasNodeID[flow.Identity](executorIDs...)).ToSkeleton(), nil
}

// ChooseFromPreferredENIDs finds the subset of execution nodes if preferred execution nodes are defined.
// If preferredENIdentifiers is set and there are less than maxNodesCnt nodes selected, than the list is padded up to
// maxNodesCnt nodes using the following order:
// 1. Use any EN with a receipt.
// 2. Use any preferred node not already selected.
// 3. Use any EN not already selected.
func ChooseFromPreferredENIDs(allENs flow.IdentityList,
	executorIDs flow.IdentifierList,
	preferredENIdentifiers flow.IdentifierList,
) flow.IdentityList {
	var chosenIDs flow.IdentityList

	// filter for both preferred and executor IDs
	chosenIDs = allENs.Filter(filter.And(
		filter.HasNodeID[flow.Identity](preferredENIdentifiers...),
		filter.HasNodeID[flow.Identity](executorIDs...),
	))

	if len(chosenIDs) >= MaxNodesCnt {
		return chosenIDs
	}

	// function to add nodes to chosenIDs if they are not already included
	addIfNotExists := func(candidates flow.IdentityList) {
		for _, en := range candidates {
			_, exists := chosenIDs.ByNodeID(en.NodeID)
			if !exists {
				chosenIDs = append(chosenIDs, en)
				if len(chosenIDs) >= MaxNodesCnt {
					return
				}
			}
		}
	}

	// add any EN with a receipt
	receiptENs := allENs.Filter(filter.HasNodeID[flow.Identity](executorIDs...))
	addIfNotExists(receiptENs)
	if len(chosenIDs) >= MaxNodesCnt {
		return chosenIDs
	}

	// add any preferred node not already selected
	preferredENs := allENs.Filter(filter.HasNodeID[flow.Identity](preferredENIdentifiers...))
	addIfNotExists(preferredENs)
	if len(chosenIDs) >= MaxNodesCnt {
		return chosenIDs
	}

	// add any EN not already selected
	addIfNotExists(allENs)

	return chosenIDs
}
