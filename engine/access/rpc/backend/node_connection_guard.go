package backend

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/storage"
	"time"

	"github.com/rs/zerolog"
	"github.com/sony/gobreaker"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

type NodeSelector interface {
	GetExecutionNodesForBlockID(ctx context.Context, blockID flow.Identifier) (flow.IdentityList, error)
	GetCollectionNodes(txID flow.Identifier) ([]string, error)
}

type NodeConnectionGuard struct {
	state             protocol.State
	executionReceipts storage.ExecutionReceipts
	log               zerolog.Logger
	circuitBreaker    *gobreaker.CircuitBreaker
	connectionFactory ConnectionFactory
}

var _ NodeSelector = (*NodeConnectionGuard)(nil)

func NewNodeConnectionGuard(connectionFactory ConnectionFactory, state protocol.State, executionReceipts storage.ExecutionReceipts, log zerolog.Logger) NodeConnectionGuard {
	return NodeConnectionGuard{
		state:             state,
		executionReceipts: executionReceipts,
		log:               log,
		circuitBreaker:    gobreaker.NewCircuitBreaker(gobreaker.Settings{}),
		connectionFactory: connectionFactory,
	}
}

func (ncg *NodeConnectionGuard) Invoke(req func() (interface{}, error)) (interface{}, error) {
	result, err := ncg.circuitBreaker.Execute(req)
	return result, err
}

func (ncg *NodeConnectionGuard) GetCollectionNodes(txId flow.Identifier) ([]string, error) {
	// retrieve the set of collector clusters
	clusters, err := ncg.state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(txId)
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %x", txId)
	}

	// select a random subset of collection nodes from the cluster to be tried in order
	//TODO: Change to cb selection of nodes.
	targetNodes := txCluster.Sample(3)

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
func (ncg *NodeConnectionGuard) GetExecutionNodesForBlockID(
	ctx context.Context,
	blockID flow.Identifier) (flow.IdentityList, error) {

	var executorIDs flow.IdentifierList

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	rootBlock, err := ncg.state.Params().Root()
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	if rootBlock.ID() == blockID {
		executorIdentities, err := ncg.state.Final().Identities(filter.HasRole(flow.RoleExecution))
		if err != nil {
			return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
		}
		executorIDs = executorIdentities.NodeIDs()
	} else {
		// try to find atleast minExecutionNodesCnt execution node ids from the execution receipts for the given blockID
		for attempt := 0; attempt < maxAttemptsForExecutionReceipt; attempt++ {
			executorIDs, err = ncg.findAllExecutionNodes(blockID)
			if err != nil {
				return nil, err
			}

			if len(executorIDs) >= minExecutionNodesCnt {
				break
			}

			// log the attempt
			ncg.log.Debug().Int("attempt", attempt).Int("max_attempt", maxAttemptsForExecutionReceipt).
				Int("execution_receipts_found", len(executorIDs)).
				Str("block_id", blockID.String()).
				Msg("insufficient execution receipts")

			// if one or less execution receipts may have been received then re-query
			// in the hope that more might have been received by now
			//TODO: Should be removed
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond << time.Duration(attempt)):
				//retry after an exponential backoff
			}
		}

		receiptCnt := len(executorIDs)
		// if less than minExecutionNodesCnt execution receipts have been received so far, then return random ENs
		if receiptCnt < minExecutionNodesCnt {
			newExecutorIDs, err := ncg.state.AtBlockID(blockID).Identities(filter.HasRole(flow.RoleExecution))
			if err != nil {
				return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
			}
			executorIDs = newExecutorIDs.NodeIDs()
		}
	}

	// choose from the preferred or fixed execution nodes
	subsetENs, err := ncg.chooseExecutionNodes(executorIDs)
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

// findAllExecutionNodes find all the execution nodes ids from the execution receipts that have been received for the
// given blockID
func (ncg *NodeConnectionGuard) findAllExecutionNodes(
	blockID flow.Identifier) (flow.IdentifierList, error) {

	// lookup the receipt's storage with the block ID
	allReceipts, err := ncg.executionReceipts.ByBlockID(blockID)
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
		ncg.log.Error().
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
func (ncg *NodeConnectionGuard) chooseExecutionNodes(executorIDs flow.IdentifierList) (flow.IdentityList, error) {

	allENs, err := ncg.state.Final().Identities(filter.HasRole(flow.RoleExecution))
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
