package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// maxExecutionNodesCnt is the max number of execution nodes that will be contacted to complete an execution api request
const maxExecutionNodesCnt = 3

// minExecutionNodesCnt is the minimum number of execution nodes expected to have sent the execution receipt for a block
const minExecutionNodesCnt = 2

// maxAttemptsForExecutionReceipt is the maximum number of attempts to find execution receipts for a given block ID
const maxAttemptsForExecutionReceipt = 3

// DefaultMaxHeightRange is the default maximum size of range requests.
const DefaultMaxHeightRange = 250

var preferredENIdentifiers flow.IdentifierList
var fixedENIdentifiers flow.IdentifierList

// Backends implements the Access API.
//
// It is composed of several sub-backends that implement part of the Access API.
//
// Script related calls are handled by backendScripts.
// Transaction related calls are handled by backendTransactions.
// Block Header related calls are handled by backendBlockHeaders.
// Block details related calls are handled by backendBlockDetails.
// Event related calls are handled by backendEvents.
// Account related calls are handled by backendAccounts.
//
// All remaining calls are handled by the base Backend in this file.
type Backend struct {
	backendScripts
	backendTransactions
	backendEvents
	backendBlockHeaders
	backendBlockDetails
	backendAccounts
	backendExecutionResults

	state             protocol.State
	chainID           flow.ChainID
	collections       storage.Collections
	executionReceipts storage.ExecutionReceipts
	connFactory       ConnectionFactory
}

func New(
	state protocol.State,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
	executionResults storage.ExecutionResults,
	chainID flow.ChainID,
	transactionMetrics module.TransactionMetrics,
	connFactory ConnectionFactory,
	retryEnabled bool,
	maxHeightRange uint,
	preferredExecutionNodeIDs []string,
	fixedExecutionNodeIDs []string,
	log zerolog.Logger,
) *Backend {
	retry := newRetry()
	if retryEnabled {
		retry.Activate()
	}

	b := &Backend{
		state: state,
		// create the sub-backends
		backendScripts: backendScripts{
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			state:             state,
			log:               log,
		},
		backendTransactions: backendTransactions{
			staticCollectionRPC:  collectionRPC,
			state:                state,
			chainID:              chainID,
			collections:          collections,
			blocks:               blocks,
			transactions:         transactions,
			executionReceipts:    executionReceipts,
			transactionValidator: configureTransactionValidator(state, chainID),
			transactionMetrics:   transactionMetrics,
			retry:                retry,
			connFactory:          connFactory,
			previousAccessNodes:  historicalAccessNodes,
			log:                  log,
		},
		backendEvents: backendEvents{
			state:             state,
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			log:               log,
			maxHeightRange:    maxHeightRange,
		},
		backendBlockHeaders: backendBlockHeaders{
			headers: headers,
			state:   state,
		},
		backendBlockDetails: backendBlockDetails{
			blocks: blocks,
			state:  state,
		},
		backendAccounts: backendAccounts{
			state:             state,
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			log:               log,
		},
		backendExecutionResults: backendExecutionResults{
			executionResults: executionResults,
		},
		collections:       collections,
		executionReceipts: executionReceipts,
		connFactory:       connFactory,
		chainID:           chainID,
	}

	retry.SetBackend(b)

	var err error
	preferredENIdentifiers, err = identifierList(preferredExecutionNodeIDs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert node id string to Flow Identifier for preferred EN map")
	}

	fixedENIdentifiers, err = identifierList(fixedExecutionNodeIDs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert node id string to Flow Identifier for fixed EN map")
	}

	return b
}

func identifierList(ids []string) (flow.IdentifierList, error) {
	idList := make(flow.IdentifierList, len(ids))
	for i, idStr := range ids {
		id, err := flow.HexStringToIdentifier(idStr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert node id string %s to Flow Identifier: %v", id, err)
		}
		idList[i] = id
	}
	return idList, nil
}

func configureTransactionValidator(state protocol.State, chainID flow.ChainID) *access.TransactionValidator {
	return access.NewTransactionValidator(
		access.NewProtocolStateBlocks(state),
		chainID.Chain(),
		access.TransactionValidationOptions{
			Expiry:                       flow.DefaultTransactionExpiry,
			ExpiryBuffer:                 flow.DefaultTransactionExpiryBuffer,
			AllowEmptyReferenceBlockID:   false,
			AllowUnknownReferenceBlockID: false,
			CheckScriptsParse:            false,
			MaxGasLimit:                  flow.DefaultMaxTransactionGasLimit,
			MaxTransactionByteSize:       flow.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flow.DefaultMaxCollectionByteSize,
		},
	)
}

// Ping responds to requests when the server is up.
func (b *Backend) Ping(ctx context.Context) error {

	// staticCollectionRPC is only set if a collection node address was provided at startup
	if b.staticCollectionRPC != nil {
		_, err := b.staticCollectionRPC.Ping(ctx, &accessproto.PingRequest{})
		if err != nil {
			return fmt.Errorf("could not ping collection node: %w", err)
		}
	}

	return nil
}

func (b *Backend) GetCollectionByID(_ context.Context, colID flow.Identifier) (*flow.LightCollection, error) {
	// retrieve the collection from the collection storage
	col, err := b.collections.LightByID(colID)
	if err != nil {
		// Collections are retrieved asynchronously as we finalize blocks, so
		// it is possible for a client to request a finalized block from us
		// containing some collection, then get a not found error when requesting
		// that collection. These clients should retry.
		err = convertStorageError(fmt.Errorf("please retry for collection in finalized block: %w", err))
		return nil, err
	}

	return col, nil
}

func (b *Backend) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: b.chainID,
	}
}

func (b *Backend) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	data, err := convert.SnapshotToBytes(b.state.Final())
	if err != nil {
		return nil, err
	}

	return data, nil
}

func convertStorageError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.NotFound {
		// Already converted
		return err
	}
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}

// executionNodesForBlockID returns upto maxExecutionNodesCnt number of randomly chosen execution node identities
// which have executed the given block ID.
// If no such execution node is found, an InsufficientExecutionReceipts error is returned.
func executionNodesForBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	state protocol.State,
	log zerolog.Logger) (flow.IdentityList, error) {

	var executorIDs flow.IdentifierList
	var err error
	attempt := 0

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	rootBlock, err := state.Params().Root()
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	if rootBlock.ID() == blockID {
		executorIdentities, err := state.Final().Identities(filter.HasRole(flow.RoleExecution))
		if err != nil {
			return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
		}
		executorIDs = executorIdentities.NodeIDs()
	} else {
		// try to find atleast minExecutionNodesCnt execution node ids from the execution receipts for the given blockID
		for ; attempt < maxAttemptsForExecutionReceipt; attempt++ {

			executorIDs, err = findAllExecutionNodes(blockID, executionReceipts, log)
			if err != nil {
				return flow.IdentityList{}, err
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
				return flow.IdentityList{}, err
			case <-time.After(100 * time.Millisecond << time.Duration(attempt)):
				//retry after an exponential backoff
			}
		}

		receiptCnt := len(executorIDs)
		// if less than minExecutionNodesCnt execution receipts have been received so far, then throw an error
		if receiptCnt < minExecutionNodesCnt {
			return flow.IdentityList{}, InsufficientExecutionReceipts{blockID: blockID, receiptCount: receiptCnt}
		}
	}

	// choose from the preferred or fixed execution nodes
	subsetENs, err := chooseExecutionNodes(state, executorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	// randomly choose upto maxExecutionNodesCnt identities
	executionIdentitiesRandom := subsetENs.Sample(maxExecutionNodesCnt)

	if len(executionIdentitiesRandom) == 0 {
		return flow.IdentityList{},
			fmt.Errorf("no matching execution node could for block ID %v", blockID)
	}

	return executionIdentitiesRandom, nil
}

// findAllExecutionNodes find all the execution nodes ids from the execution receipts that have been received for the
// given blockID
func findAllExecutionNodes(
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	log zerolog.Logger) (flow.IdentifierList, error) {

	// lookup the receipts storage with the block ID
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
