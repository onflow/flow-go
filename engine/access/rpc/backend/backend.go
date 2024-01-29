package backend

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// minExecutionNodesCnt is the minimum number of execution nodes expected to have sent the execution receipt for a block
const minExecutionNodesCnt = 2

// maxAttemptsForExecutionReceipt is the maximum number of attempts to find execution receipts for a given block ID
const maxAttemptsForExecutionReceipt = 3

// DefaultMaxHeightRange is the default maximum size of range requests.
const DefaultMaxHeightRange = 250

// DefaultSnapshotHistoryLimit the amount of blocks to look back in state
// when recursively searching for a valid snapshot
const DefaultSnapshotHistoryLimit = 500

// DefaultLoggedScriptsCacheSize is the default size of the lookup cache used to dedupe logs of scripts sent to ENs
// limiting cache size to 16MB and does not affect script execution, only for keeping logs tidy
const DefaultLoggedScriptsCacheSize = 1_000_000

// DefaultConnectionPoolSize is the default size for the connection pool to collection and execution nodes
const DefaultConnectionPoolSize = 250

var preferredENIdentifiers flow.IdentifierList
var fixedENIdentifiers flow.IdentifierList

// Backend implements the Access API.
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
	backendNetwork

	state             protocol.State
	chainID           flow.ChainID
	collections       storage.Collections
	executionReceipts storage.ExecutionReceipts
	connFactory       connection.ConnectionFactory

	// cache the response to GetNodeVersionInfo since it doesn't change
	nodeInfo *access.NodeVersionInfo
}

type Params struct {
	State                     protocol.State
	CollectionRPC             accessproto.AccessAPIClient
	HistoricalAccessNodes     []accessproto.AccessAPIClient
	Blocks                    storage.Blocks
	Headers                   storage.Headers
	Events                    storage.Events
	Collections               storage.Collections
	Transactions              storage.Transactions
	ExecutionReceipts         storage.ExecutionReceipts
	ExecutionResults          storage.ExecutionResults
	LightTransactionResults   storage.LightTransactionResults
	ChainID                   flow.ChainID
	AccessMetrics             module.AccessMetrics
	ConnFactory               connection.ConnectionFactory
	RetryEnabled              bool
	MaxHeightRange            uint
	PreferredExecutionNodeIDs []string
	FixedExecutionNodeIDs     []string
	Log                       zerolog.Logger
	SnapshotHistoryLimit      int
	Communicator              Communicator
	TxResultCacheSize         uint
	TxErrorMessagesCacheSize  uint
	ScriptExecutor            execution.ScriptExecutor
	ScriptExecutionMode       IndexQueryMode
	EventQueryMode            IndexQueryMode
}

// New creates backend instance
func New(params Params) (*Backend, error) {
	retry := newRetry(params.Log)
	if params.RetryEnabled {
		retry.Activate()
	}

	loggedScripts, err := lru.New[[md5.Size]byte, time.Time](DefaultLoggedScriptsCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize script logging cache: %w", err)
	}

	var txResCache *lru.Cache[flow.Identifier, *access.TransactionResult]
	if params.TxResultCacheSize > 0 {
		txResCache, err = lru.New[flow.Identifier, *access.TransactionResult](int(params.TxResultCacheSize))
		if err != nil {
			return nil, fmt.Errorf("failed to init cache for transaction results: %w", err)
		}
	}

	// NOTE: The transaction error message cache is currently only used by the access node and not by the observer node.
	//       To avoid introducing unnecessary command line arguments in the observer, one case could be that the error
	//       message cache is nil for the observer node.
	var txErrorMessagesCache *lru.Cache[flow.Identifier, string]

	if params.TxErrorMessagesCacheSize > 0 {
		txErrorMessagesCache, err = lru.New[flow.Identifier, string](int(params.TxErrorMessagesCacheSize))
		if err != nil {
			return nil, fmt.Errorf("failed to init cache for transaction error messages: %w", err)
		}
	}

	// initialize node version info
	nodeInfo, err := getNodeVersionInfo(params.State.Params())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize node version info: %w", err)
	}

	b := &Backend{
		state: params.State,
		// create the sub-backends
		backendScripts: backendScripts{
			log:               params.Log,
			headers:           params.Headers,
			executionReceipts: params.ExecutionReceipts,
			connFactory:       params.ConnFactory,
			state:             params.State,
			metrics:           params.AccessMetrics,
			loggedScripts:     loggedScripts,
			nodeCommunicator:  params.Communicator,
			scriptExecutor:    params.ScriptExecutor,
			scriptExecMode:    params.ScriptExecutionMode,
		},
		backendTransactions: backendTransactions{
			log:                  params.Log,
			staticCollectionRPC:  params.CollectionRPC,
			state:                params.State,
			chainID:              params.ChainID,
			collections:          params.Collections,
			blocks:               params.Blocks,
			transactions:         params.Transactions,
			results:              params.LightTransactionResults,
			executionReceipts:    params.ExecutionReceipts,
			transactionValidator: configureTransactionValidator(params.State, params.ChainID),
			transactionMetrics:   params.AccessMetrics,
			retry:                retry,
			connFactory:          params.ConnFactory,
			previousAccessNodes:  params.HistoricalAccessNodes,
			nodeCommunicator:     params.Communicator,
			txResultCache:        txResCache,
			txErrorMessagesCache: txErrorMessagesCache,
		},
		backendEvents: backendEvents{
			log:               params.Log,
			chain:             params.ChainID.Chain(),
			state:             params.State,
			headers:           params.Headers,
			events:            params.Events,
			executionReceipts: params.ExecutionReceipts,
			connFactory:       params.ConnFactory,
			maxHeightRange:    params.MaxHeightRange,
			nodeCommunicator:  params.Communicator,
			queryMode:         params.EventQueryMode,
		},
		backendBlockHeaders: backendBlockHeaders{
			headers: params.Headers,
			state:   params.State,
		},
		backendBlockDetails: backendBlockDetails{
			blocks: params.Blocks,
			state:  params.State,
		},
		backendAccounts: backendAccounts{
			log:               params.Log,
			state:             params.State,
			headers:           params.Headers,
			executionReceipts: params.ExecutionReceipts,
			connFactory:       params.ConnFactory,
			nodeCommunicator:  params.Communicator,
			scriptExecutor:    params.ScriptExecutor,
			scriptExecMode:    params.ScriptExecutionMode,
		},
		backendExecutionResults: backendExecutionResults{
			executionResults: params.ExecutionResults,
		},
		backendNetwork: backendNetwork{
			state:                params.State,
			chainID:              params.ChainID,
			headers:              params.Headers,
			snapshotHistoryLimit: params.SnapshotHistoryLimit,
		},
		collections:       params.Collections,
		executionReceipts: params.ExecutionReceipts,
		connFactory:       params.ConnFactory,
		chainID:           params.ChainID,
		nodeInfo:          nodeInfo,
	}

	retry.SetBackend(b)

	preferredENIdentifiers, err = identifierList(params.PreferredExecutionNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for preferred EN map: %w", err)
	}

	fixedENIdentifiers, err = identifierList(params.FixedExecutionNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for fixed EN map: %w", err)
	}

	return b, nil
}

// NewCache constructs cache for storing connections to other nodes.
// No errors are expected during normal operations.
func NewCache(
	log zerolog.Logger,
	metrics module.AccessMetrics,
	connectionPoolSize int,
) (*lru.Cache[string, *connection.CachedClient], error) {
	cache, err := lru.NewWithEvict(connectionPoolSize, func(_ string, client *connection.CachedClient) {
		go client.Close() // close is blocking, so run in a goroutine

		log.Debug().Str("grpc_conn_evicted", client.Address).Msg("closing grpc connection evicted from pool")
		metrics.ConnectionFromPoolEvicted()
	})

	if err != nil {
		return nil, fmt.Errorf("could not initialize connection pool cache: %w", err)
	}

	return cache, nil
}

func identifierList(ids []string) (flow.IdentifierList, error) {
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

// GetNodeVersionInfo returns node version information such as semver, commit, sporkID, protocolVersion, etc
func (b *Backend) GetNodeVersionInfo(_ context.Context) (*access.NodeVersionInfo, error) {
	return b.nodeInfo, nil
}

// getNodeVersionInfo returns the NodeVersionInfo for the node.
// Since these values are static while the node is running, it is safe to cache.
func getNodeVersionInfo(stateParams protocol.Params) (*access.NodeVersionInfo, error) {
	sporkID, err := stateParams.SporkID()
	if err != nil {
		return nil, fmt.Errorf("failed to read spork ID: %v", err)
	}

	protocolVersion, err := stateParams.ProtocolVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to read protocol version: %v", err)
	}

	sporkRootBlockHeight, err := stateParams.SporkRootBlockHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to read spork root block height: %w", err)
	}

	nodeRootBlockHeader, err := stateParams.SealedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to read node root block: %w", err)
	}

	nodeInfo := &access.NodeVersionInfo{
		Semver:               build.Version(),
		Commit:               build.Commit(),
		SporkId:              sporkID,
		ProtocolVersion:      uint64(protocolVersion),
		SporkRootBlockHeight: sporkRootBlockHeight,
		NodeRootBlockHeight:  nodeRootBlockHeader.Height,
	}

	return nodeInfo, nil
}

func (b *Backend) GetCollectionByID(_ context.Context, colID flow.Identifier) (*flow.LightCollection, error) {
	// retrieve the collection from the collection storage
	col, err := b.collections.LightByID(colID)
	if err != nil {
		// Collections are retrieved asynchronously as we finalize blocks, so
		// it is possible for a client to request a finalized block from us
		// containing some collection, then get a not found error when requesting
		// that collection. These clients should retry.
		err = rpc.ConvertStorageError(fmt.Errorf("please retry for collection in finalized block: %w", err))
		return nil, err
	}

	return col, nil
}

func (b *Backend) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: b.chainID,
	}
}

// executionNodesForBlockID returns upto maxNodesCnt number of randomly chosen execution node identities
// which have executed the given block ID.
// If no such execution node is found, an InsufficientExecutionReceipts error is returned.
func executionNodesForBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	state protocol.State,
	log zerolog.Logger,
) (flow.IdentityList, error) {

	var executorIDs flow.IdentifierList

	// check if the block ID is of the root block. If it is then don't look for execution receipts since they
	// will not be present for the root block.
	rootBlock, err := state.Params().FinalizedRoot()
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
				//retry after an exponential backoff
			}
		}

		receiptCnt := len(executorIDs)
		// if less than minExecutionNodesCnt execution receipts have been received so far, then return random ENs
		if receiptCnt < minExecutionNodesCnt {
			newExecutorIDs, err := state.AtBlockID(blockID).Identities(filter.HasRole(flow.RoleExecution))
			if err != nil {
				return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
			}
			executorIDs = newExecutorIDs.NodeIDs()
		}
	}

	// choose from the preferred or fixed execution nodes
	subsetENs, err := chooseExecutionNodes(state, executorIDs)
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
