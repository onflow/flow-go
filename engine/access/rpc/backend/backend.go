package backend

import (
	"context"
	"errors"
	"fmt"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// maxExecutionNodesCnt is the max number of execution nodes that will be contacted to complete an execution api request
const maxExecutionNodesCnt = 3

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

	executionRPC      execproto.ExecutionAPIClient
	state             protocol.State
	chainID           flow.ChainID
	collections       storage.Collections
	executionReceipts storage.ExecutionReceipts
	connFactory       ConnectionFactory
}

func New(
	state protocol.State,
	executionRPC execproto.ExecutionAPIClient,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
	chainID flow.ChainID,
	transactionMetrics module.TransactionMetrics,
	connFactory ConnectionFactory,
	retryEnabled bool,
	preferredExecutionNodeIDs []string,
	fixedExecutionNodeIDs []string,
	log zerolog.Logger,
) *Backend {
	retry := newRetry()
	if retryEnabled {
		retry.Activate()
	}

	b := &Backend{
		executionRPC: executionRPC,
		state:        state,
		// create the sub-backends
		backendScripts: backendScripts{
			headers:            headers,
			executionReceipts:  executionReceipts,
			staticExecutionRPC: executionRPC,
			connFactory:        connFactory,
			state:              state,
			log:                log,
		},
		backendTransactions: backendTransactions{
			staticCollectionRPC:  collectionRPC,
			executionRPC:         executionRPC,
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
			staticExecutionRPC: executionRPC,
			state:              state,
			blocks:             blocks,
			executionReceipts:  executionReceipts,
			connFactory:        connFactory,
			log:                log,
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
			staticExecutionRPC: executionRPC,
			state:              state,
			headers:            headers,
			executionReceipts:  executionReceipts,
			connFactory:        connFactory,
			log:                log,
		},
		collections:       collections,
		executionReceipts: executionReceipts,
		connFactory:       connFactory,
		chainID:           chainID,
	}

	retry.SetBackend(b)

	var err error
	preferredENIdentifiers, err = enIDsToIndentifierList(preferredExecutionNodeIDs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert node id string to Flow Identifier for preferred EN map")
	}

	fixedENIdentifiers, err = enIDsToIndentifierList(fixedExecutionNodeIDs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert node id string to Flow Identifier for fixed EN map")
	}

	return b
}

func enIDsToIndentifierList(ids []string) (flow.IdentifierList, error) {
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
			MaxGasLimit:                  flow.DefaultMaxGasLimit,
			CheckScriptsParse:            true,
			MaxTxSizeLimit:               flow.DefaultMaxTxSizeLimit,
		},
	)
}

// Ping responds to requests when the server is up.
func (b *Backend) Ping(ctx context.Context) error {
	_, err := b.executionRPC.Ping(ctx, &execproto.PingRequest{})
	if err != nil {
		return fmt.Errorf("could not ping execution node: %w", err)
	}

	// staticCollectionRPC is only set if a collection node address was provided at startup
	if b.staticCollectionRPC != nil {
		_, err = b.staticCollectionRPC.Ping(ctx, &accessproto.PingRequest{})
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
		err = convertStorageError(err)
		return nil, err
	}

	return col, nil
}

func (b *Backend) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: b.chainID,
	}
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
// which have executed the given block ID. If no such execution node is found, an empty list is returned.
func executionNodesForBlockID(
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	state protocol.State,
	log zerolog.Logger) (flow.IdentityList, error) {

	// lookup the receipts storage with the block ID
	allReceipts, err := executionReceipts.ByBlockIDAllExecutionReceipts(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	// execution result ID to execution receipt map to keep track of receipts by their result id
	var identicalReceipts = make(map[flow.Identifier][]*flow.ExecutionReceipt)

	// maximum number of matching receipts found so far for any execution result id
	maxMatchedReceiptCnt := 0
	// execution result id key for the highest number of matching receipts in the identicalReceipts map
	var maxMatchedReceiptResultID flow.Identifier

	// find the largest list of receipts which have the same result ID
	for _, receipt := range allReceipts {

		resultID := receipt.ExecutionResult.ID()
		identicalReceipts[resultID] = append(identicalReceipts[resultID], receipt)

		currentMatchedReceiptCnt := len(identicalReceipts[resultID])
		if currentMatchedReceiptCnt > maxMatchedReceiptCnt {
			maxMatchedReceiptCnt = currentMatchedReceiptCnt
			maxMatchedReceiptResultID = resultID
		}
	}

	mismatchReceiptCnt := len(identicalReceipts)
	// if there are more than one execution result for the same block ID, log as error
	if mismatchReceiptCnt > 1 {
		identicalReceiptsStr := fmt.Sprintf("%v", flow.GetIDs(allReceipts))
		log.Error().
			Str("block_id", blockID.String()).
			Str("execution_receipts", identicalReceiptsStr).
			Msg("execution receipt mismatch")
	}

	// pick the largest list of matching receipts
	matchingReceipts := identicalReceipts[maxMatchedReceiptResultID]

	// collect all unique execution node ids from the receipts
	var executorIDs flow.IdentifierList
	executorIDMap := make(map[flow.Identifier]bool)
	for _, receipt := range matchingReceipts {
		if executorIDMap[receipt.ExecutorID] {
			continue
		}
		executorIDs = append(executorIDs, receipt.ExecutorID)
		executorIDMap[receipt.ExecutorID] = true
	}

	// return if less than 2 unique execution node ids were found
	// since we want matching receipts from at least 2 ENs
	if len(executorIDs) < 2 {
		return flow.IdentityList{}, nil
	}

	// choose one of the preferred execution nodes
	subsetENs, err := chooseExecutionNodes(state, executorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	// randomly choose upto maxExecutionNodesCnt identities
	executionIdentitiesRandom := subsetENs.Sample(maxExecutionNodesCnt)

	return executionIdentitiesRandom, nil
}

func chooseExecutionNodes(state protocol.State, executorIDs flow.IdentifierList) (flow.IdentityList, error) {

	allENs, err := state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return nil, fmt.Errorf("failed to retreive all execution IDs: %w", err)
	}

	// find the preferred execution node IDs which have executed the transaction
	preferredENIDs := allENs.Filter(filter.And(filter.HasNodeID(preferredENIdentifiers...),
		filter.HasNodeID(executorIDs...)))

	if len(preferredENIDs) > 0 {
		return preferredENIDs, nil
	}

	// if no such preferred ID is found, the choose the fixed EN IDs
	fixedENIDs := allENs.Filter(filter.HasNodeID(fixedENIdentifiers...))

	return fixedENIDs, nil
}
