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
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

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

var (
	preferredENIdentifiers flow.IdentifierList
	fixedENIdentifiers     flow.IdentifierList
)

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
	backendSubscribeBlocks
	backendSubscribeTransactions

	state             protocol.State
	chainID           flow.ChainID
	collections       storage.Collections
	executionReceipts storage.ExecutionReceipts
	connFactory       connection.ConnectionFactory

	BlockTracker   subscription.BlockTracker
	stateParams    protocol.Params
	versionControl *version.VersionControl
}

type Params struct {
	State                     protocol.State
	CollectionRPC             accessproto.AccessAPIClient
	HistoricalAccessNodes     []accessproto.AccessAPIClient
	Blocks                    storage.Blocks
	Headers                   storage.Headers
	Collections               storage.Collections
	Transactions              storage.Transactions
	ExecutionReceipts         storage.ExecutionReceipts
	ExecutionResults          storage.ExecutionResults
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
	CheckPayerBalance         bool
	EventQueryMode            IndexQueryMode
	BlockTracker              subscription.BlockTracker
	SubscriptionHandler       *subscription.SubscriptionHandler

	EventsIndex         *index.EventsIndex
	TxResultQueryMode   IndexQueryMode
	TxResultsIndex      *index.TransactionResultsIndex
	LastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	VersionControl      *version.VersionControl
}

var _ TransactionErrorMessage = (*Backend)(nil)

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

	// the system tx is hardcoded and never changes during runtime
	systemTx, err := blueprints.SystemChunkTransaction(params.ChainID.Chain())
	if err != nil {
		return nil, fmt.Errorf("failed to create system chunk transaction: %w", err)
	}
	systemTxID := systemTx.ID()

	transactionsLocalDataProvider := &TransactionsLocalDataProvider{
		state:               params.State,
		collections:         params.Collections,
		blocks:              params.Blocks,
		eventsIndex:         params.EventsIndex,
		txResultsIndex:      params.TxResultsIndex,
		systemTxID:          systemTxID,
		lastFullBlockHeight: params.LastFullBlockHeight,
	}

	b := &Backend{
		state:        params.State,
		BlockTracker: params.BlockTracker,
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
		backendEvents: backendEvents{
			log:               params.Log,
			chain:             params.ChainID.Chain(),
			state:             params.State,
			headers:           params.Headers,
			executionReceipts: params.ExecutionReceipts,
			connFactory:       params.ConnFactory,
			maxHeightRange:    params.MaxHeightRange,
			nodeCommunicator:  params.Communicator,
			queryMode:         params.EventQueryMode,
			eventsIndex:       params.EventsIndex,
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
		backendSubscribeBlocks: backendSubscribeBlocks{
			log:                 params.Log,
			state:               params.State,
			headers:             params.Headers,
			blocks:              params.Blocks,
			subscriptionHandler: params.SubscriptionHandler,
			blockTracker:        params.BlockTracker,
		},

		collections:       params.Collections,
		executionReceipts: params.ExecutionReceipts,
		connFactory:       params.ConnFactory,
		chainID:           params.ChainID,
		stateParams:       params.State.Params(),
		versionControl:    params.VersionControl,
	}

	txValidator, err := configureTransactionValidator(params.State, params.ChainID, params.AccessMetrics, params.ScriptExecutor, params.CheckPayerBalance)
	if err != nil {
		return nil, fmt.Errorf("could not create transaction validator: %w", err)
	}

	b.backendTransactions = backendTransactions{
		TransactionsLocalDataProvider: transactionsLocalDataProvider,
		log:                           params.Log,
		staticCollectionRPC:           params.CollectionRPC,
		chainID:                       params.ChainID,
		transactions:                  params.Transactions,
		executionReceipts:             params.ExecutionReceipts,
		transactionValidator:          txValidator,
		transactionMetrics:            params.AccessMetrics,
		retry:                         retry,
		connFactory:                   params.ConnFactory,
		previousAccessNodes:           params.HistoricalAccessNodes,
		nodeCommunicator:              params.Communicator,
		txResultCache:                 txResCache,
		txErrorMessagesCache:          txErrorMessagesCache,
		txResultQueryMode:             params.TxResultQueryMode,
		systemTx:                      systemTx,
		systemTxID:                    systemTxID,
	}

	// TODO: The TransactionErrorMessage interface should be reorganized in future, as it is implemented in backendTransactions but used in TransactionsLocalDataProvider, and its initialization is somewhat quirky.
	b.backendTransactions.txErrorMessages = b

	b.backendSubscribeTransactions = backendSubscribeTransactions{
		txLocalDataProvider: transactionsLocalDataProvider,
		backendTransactions: &b.backendTransactions,
		log:                 params.Log,
		executionResults:    params.ExecutionResults,
		subscriptionHandler: params.SubscriptionHandler,
		blockTracker:        params.BlockTracker,
	}

	retry.SetBackend(b)

	preferredENIdentifiers, err = commonrpc.IdentifierList(params.PreferredExecutionNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for preferred EN map: %w", err)
	}

	fixedENIdentifiers, err = commonrpc.IdentifierList(params.FixedExecutionNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for fixed EN map: %w", err)
	}

	return b, nil
}

func configureTransactionValidator(
	state protocol.State,
	chainID flow.ChainID,
	transactionMetrics module.TransactionValidationMetrics,
	executor execution.ScriptExecutor,
	checkPayerBalance bool,
) (*access.TransactionValidator, error) {
	return access.NewTransactionValidator(
		access.NewProtocolStateBlocks(state),
		chainID.Chain(),
		transactionMetrics,
		access.TransactionValidationOptions{
			Expiry:                       flow.DefaultTransactionExpiry,
			ExpiryBuffer:                 flow.DefaultTransactionExpiryBuffer,
			AllowEmptyReferenceBlockID:   false,
			AllowUnknownReferenceBlockID: false,
			CheckScriptsParse:            false,
			MaxGasLimit:                  flow.DefaultMaxTransactionGasLimit,
			MaxTransactionByteSize:       flow.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flow.DefaultMaxCollectionByteSize,
			CheckPayerBalance:            checkPayerBalance,
		},
		executor,
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
	sporkID := b.stateParams.SporkID()
	protocolVersion := b.stateParams.ProtocolVersion()
	sporkRootBlockHeight := b.stateParams.SporkRootBlockHeight()

	nodeRootBlockHeader := b.stateParams.SealedRoot()

	var compatibleRange *access.CompatibleRange

	// Version control feature could be disabled
	if b.versionControl != nil {
		compatibleRange = &access.CompatibleRange{
			StartHeight: b.versionControl.StartHeight(),
			EndHeight:   b.versionControl.EndHeight(),
		}
	}

	nodeInfo := &access.NodeVersionInfo{
		Semver:               build.Version(),
		Commit:               build.Commit(),
		SporkId:              sporkID,
		ProtocolVersion:      uint64(protocolVersion),
		SporkRootBlockHeight: sporkRootBlockHeight,
		NodeRootBlockHeight:  nodeRootBlockHeader.Height,
		CompatibleRange:      compatibleRange,
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

func (b *Backend) GetFullCollectionByID(_ context.Context, colID flow.Identifier) (*flow.Collection, error) {
	// retrieve the collection from the collection storage
	col, err := b.collections.ByID(colID)
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
