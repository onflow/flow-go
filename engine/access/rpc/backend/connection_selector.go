package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

const collectionNodesToTry uint = 3

type ConnectionSelector interface {
	GetExecutionNodesForBlockID(ctx context.Context, blockID flow.Identifier) (flow.IdentityList, error)
	GetCollectionNodes(txID flow.Identifier) ([]string, error)
}

type MainConnectionSelector struct {
	state             protocol.State
	executionReceipts storage.ExecutionReceipts
	log               zerolog.Logger
}

type CircuitBreakerConnectionSelector MainConnectionSelector

var _ ConnectionSelector = (*MainConnectionSelector)(nil)

func NewConnectionSelector(
	state protocol.State,
	executionReceipts storage.ExecutionReceipts,
	log zerolog.Logger,
	isCircuitBreakerEnabled bool,
) ConnectionSelector {
	if isCircuitBreakerEnabled {
		return &CircuitBreakerConnectionSelector{
			state:             state,
			executionReceipts: executionReceipts,
			log:               log,
		}
	} else {
		return &MainConnectionSelector{
			state:             state,
			executionReceipts: executionReceipts,
			log:               log,
		}
	}
}

func (ncs *MainConnectionSelector) GetCollectionNodes(txId flow.Identifier) ([]string, error) {
	// retrieve the set of collector clusters
	clusters, err := ncs.state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(txId)
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %x", txId)
	}

	// select a random subset of collection nodes from the cluster to be tried in order
	targetNodes := txCluster.Sample(collectionNodesToTry)

	// collect the addresses of all the chosen collection nodes
	var targetAddrs = make([]string, len(targetNodes))
	for i, id := range targetNodes {
		targetAddrs[i] = id.Address
	}

	return targetAddrs, nil
}

// GetExecutionNodesForBlockID returns upto maxExecutionNodesCnt number of randomly chosen execution node identities
// which have executed the given block ID.
// If no such execution node is found, an InsufficientExecutionReceipts error is returned.
func (ncs *MainConnectionSelector) GetExecutionNodesForBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) (flow.IdentityList, error) {

	var executorIDs flow.IdentifierList

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	rootBlock, err := ncs.state.Params().FinalizedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	if rootBlock.ID() == blockID {
		executorIdentities, err := ncs.state.Final().Identities(filter.HasRole(flow.RoleExecution))
		if err != nil {
			return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
		}
		executorIDs = executorIdentities.NodeIDs()
	} else {
		// try to find at least minExecutionNodesCnt execution node ids from the execution receipts for the given blockID
		for attempt := 0; attempt < maxAttemptsForExecutionReceipt; attempt++ {
			executorIDs, err = findAllExecutionNodes(blockID, ncs.executionReceipts, ncs.log)
			if err != nil {
				return nil, err
			}

			if len(executorIDs) > 0 {
				break
			}

			// log the attempt
			ncs.log.Debug().Int("attempt", attempt).Int("max_attempt", maxAttemptsForExecutionReceipt).
				Int("execution_receipts_found", len(executorIDs)).
				Str("block_id", blockID.String()).
				Msg("insufficient execution receipts")

			// if one or less execution receipts may have been received then re-query
			// in the hope that more might have been received by now
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond << time.Duration(attempt)):
				//retry after an exponential backoff
			}
		}
	}

	// choose from the preferred or fixed execution nodes
	subsetENs, err := chooseExecutionNodes(ncs.state, executorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	// randomly choose upto maxExecutionNodesCnt identities
	executionIdentitiesRandom := subsetENs.Sample(maxExecutionNodesCnt)

	if len(executionIdentitiesRandom) == 0 {
		return nil, fmt.Errorf("no matching execution node found for block ID %v", blockID)
	}

	return executionIdentitiesRandom, nil
}

func (nccbs *CircuitBreakerConnectionSelector) GetCollectionNodes(txId flow.Identifier) ([]string, error) {
	// retrieve the set of collector clusters
	clusters, err := nccbs.state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(txId)
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %x", txId)
	}

	// collect the addresses of all the chosen collection nodes
	var targetAddress = make([]string, len(txCluster))
	for i, id := range txCluster {
		targetAddress[i] = id.Address
	}

	return targetAddress, nil
}

// GetExecutionNodesForBlockID returns upto maxExecutionNodesCnt number of randomly chosen execution node identities
// which have executed the given block ID.
// If no such execution node is found, an InsufficientExecutionReceipts error is returned.
func (nccbs *CircuitBreakerConnectionSelector) GetExecutionNodesForBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) (flow.IdentityList, error) {

	var executorIDs flow.IdentifierList

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	rootBlock, err := nccbs.state.Params().FinalizedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	if rootBlock.ID() == blockID {
		executorIdentities, err := nccbs.state.Final().Identities(filter.HasRole(flow.RoleExecution))
		if err != nil {
			return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
		}
		executorIDs = executorIdentities.NodeIDs()
	} else {
		// try to find at least minExecutionNodesCnt execution node ids from the execution receipts for the given blockID
		for attempt := 0; attempt < maxAttemptsForExecutionReceipt; attempt++ {
			executorIDs, err = findAllExecutionNodes(blockID, nccbs.executionReceipts, nccbs.log)
			if err != nil {
				return nil, err
			}

			if len(executorIDs) > 0 {
				break
			}

			// log the attempt
			nccbs.log.Debug().Int("attempt", attempt).Int("max_attempt", maxAttemptsForExecutionReceipt).
				Int("execution_receipts_found", len(executorIDs)).
				Str("block_id", blockID.String()).
				Msg("insufficient execution receipts")

			// if one or less execution receipts may have been received then re-query
			// in the hope that more might have been received by now
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond << time.Duration(attempt)):
				//retry after an exponential backoff
			}
		}
	}

	// choose from the preferred or fixed execution nodes
	subsetENs, err := chooseExecutionNodes(nccbs.state, executorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	// randomly choose upto maxExecutionNodesCnt identities
	executionIdentitiesRandom := subsetENs.Sample(maxExecutionNodesCnt)

	if len(executionIdentitiesRandom) == 0 {
		return nil, fmt.Errorf("no matching execution node found for block ID %v", blockID)
	}

	return executionIdentitiesRandom, nil
}

// chooseExecutionNodes finds the subset of execution nodes defined in the identity table by first
// choosing the preferred execution nodes which have executed the transaction. If no such preferred
// execution nodes are found, then the fixed execution nodes defined in the identity table are returned
// If neither preferred nor fixed nodes are defined, then all execution node matching the executor IDs are returned.
// e.g. If execution nodes in identity table are {1,2,3,4}, preferred ENs are defined as {2,3,4}
// and the executor IDs is {1,2,3}, then {2, 3} is returned as the chosen subset of ENs
func chooseExecutionNodes(state protocol.State, executorIDs flow.IdentifierList) (flow.IdentityList, error) {

	allENs, err := state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retreive all execution IDs: %w", err)
	}

	// first try and choose from the preferred EN IDs
	var chosenIDs flow.IdentityList
	if len(preferredENIdentifiers) > 0 {
		// find the preferred execution node IDs which have executed the transaction
		chosenIDs = allENs.Filter(filter.And(filter.HasNodeID(preferredENIdentifiers...),
			filter.HasNodeID(executorIDs...)))
		if len(chosenIDs) > 0 {
			return chosenIDs, nil
		}
	}

	// if no preferred EN ID is found, then choose from the fixed EN IDs
	if len(fixedENIdentifiers) > 0 {
		// choose fixed ENs which have executed the transaction
		chosenIDs = allENs.Filter(filter.And(filter.HasNodeID(fixedENIdentifiers...), filter.HasNodeID(executorIDs...)))
		if len(chosenIDs) > 0 {
			return chosenIDs, nil
		}
		// if no such ENs are found then just choose all fixed ENs
		chosenIDs = allENs.Filter(filter.HasNodeID(fixedENIdentifiers...))
		return chosenIDs, nil
	}

	// If no preferred or fixed ENs have been specified, then return all executor IDs i.e. no preference at all
	return allENs.Filter(filter.HasNodeID(executorIDs...)), nil
}

// findAllExecutionNodes find all the execution nodes ids from the execution receipts that have been received for the
// given blockID
func findAllExecutionNodes(
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	log zerolog.Logger) (flow.IdentifierList, error) {

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
