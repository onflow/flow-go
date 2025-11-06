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
	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend/accounts"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/events"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	"github.com/onflow/flow-go/engine/access/rpc/backend/scripts"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	txstream "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/stream"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/version"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DefaultSnapshotHistoryLimit the amount of blocks to look back in state
// when recursively searching for a valid snapshot
const DefaultSnapshotHistoryLimit = 500

// DefaultConnectionPoolSize is the default size for the connection pool to collection and execution nodes
const DefaultConnectionPoolSize = 250

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
	accounts.Accounts
	events.Events
	scripts.Scripts
	transactions.Transactions
	txstream.TransactionStream
	backendBlockHeaders
	backendBlockDetails
	backendExecutionResults
	backendNetwork
	backendSubscribeBlocks

	state               protocol.State
	collections         storage.Collections
	staticCollectionRPC accessproto.AccessAPIClient

	stateParams    protocol.Params
	versionControl *version.VersionControl

	BlockTracker tracker.BlockTracker
}

type Params struct {
	State                    protocol.State
	CollectionRPC            accessproto.AccessAPIClient
	HistoricalAccessNodes    []accessproto.AccessAPIClient
	Blocks                   storage.Blocks
	Headers                  storage.Headers
	Collections              storage.Collections
	Transactions             storage.Transactions
	ExecutionReceipts        storage.ExecutionReceipts
	ExecutionResults         storage.ExecutionResults
	TxResultErrorMessages    storage.TransactionResultErrorMessages
	ScheduledTransactions    storage.ScheduledTransactionsReader
	ChainID                  flow.ChainID
	AccessMetrics            module.AccessMetrics
	ConnFactory              connection.ConnectionFactory
	MaxHeightRange           uint
	Log                      zerolog.Logger
	SnapshotHistoryLimit     int
	Communicator             node_communicator.Communicator
	TxResultCacheSize        uint
	ScriptExecutor           execution.ScriptExecutor
	ScriptExecutionMode      query_mode.IndexQueryMode
	CheckPayerBalanceMode    validator.PayerBalanceMode
	EventQueryMode           query_mode.IndexQueryMode
	BlockTracker             tracker.BlockTracker
	SubscriptionHandler      *subscription.SubscriptionHandler
	MaxScriptAndArgumentSize uint

	EventsIndex                *index.EventsIndex
	TxResultQueryMode          query_mode.IndexQueryMode
	TxResultsIndex             *index.TransactionResultsIndex
	LastFullBlockHeight        *counters.PersistentStrictMonotonicCounter
	IndexReporter              state_synchronization.IndexReporter
	VersionControl             *version.VersionControl
	ExecNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
	TxErrorMessageProvider     error_messages.Provider
	ScheduledCallbacksEnabled  bool
}

var _ access.API = (*Backend)(nil)

// New creates backend instance
func New(params Params) (*Backend, error) {
	loggedScripts, err := lru.New[[md5.Size]byte, time.Time](common.DefaultLoggedScriptsCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize script logging cache: %w", err)
	}

	var txResCache transactions.TxResultCache = transactions.NewNoopTxResultCache()
	if params.TxResultCacheSize > 0 {
		txResCache, err = lru.New[flow.Identifier, *accessmodel.TransactionResult](int(params.TxResultCacheSize))
		if err != nil {
			return nil, fmt.Errorf("failed to init cache for transaction results: %w", err)
		}
	}

	systemCollection, err := accessmodel.NewVersioned(
		accessmodel.VersionedSystemCollections,
		accessmodel.HardcodedChainHeightVersions[params.ChainID],
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create versioned system collections: %w", err)
	}

	accountsBackend, err := accounts.NewAccountsBackend(
		params.Log,
		params.State,
		params.Headers,
		params.ConnFactory,
		params.Communicator,
		params.ScriptExecutionMode,
		params.ScriptExecutor,
		params.ExecNodeIdentitiesProvider,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts: %w", err)
	}

	eventsBackend, err := events.NewEventsBackend(
		params.Log,
		params.State,
		params.ChainID.Chain(),
		params.MaxHeightRange,
		params.Headers,
		params.ConnFactory,
		params.Communicator,
		params.EventQueryMode,
		params.EventsIndex,
		params.ExecNodeIdentitiesProvider,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create events: %w", err)
	}

	scriptsBackend, err := scripts.NewScriptsBackend(
		params.Log,
		params.AccessMetrics,
		params.Headers,
		params.State,
		params.ConnFactory,
		params.Communicator,
		params.ScriptExecutor,
		params.ScriptExecutionMode,
		params.ExecNodeIdentitiesProvider,
		loggedScripts,
		params.MaxScriptAndArgumentSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scripts: %w", err)
	}

	txValidator, err := validator.NewTransactionValidator(
		validator.NewProtocolStateBlocks(params.State, params.IndexReporter),
		params.ChainID.Chain(),
		params.AccessMetrics,
		validator.TransactionValidationOptions{
			Expiry:                       flow.DefaultTransactionExpiry,
			ExpiryBuffer:                 flow.DefaultTransactionExpiryBuffer,
			AllowEmptyReferenceBlockID:   false,
			AllowUnknownReferenceBlockID: false,
			CheckScriptsParse:            false,
			MaxGasLimit:                  flow.DefaultMaxTransactionGasLimit,
			MaxTransactionByteSize:       flow.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flow.DefaultMaxCollectionByteSize,
			CheckPayerBalanceMode:        params.CheckPayerBalanceMode,
		},
		params.ScriptExecutor,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create transaction validator: %w", err)
	}

	txStatusDeriver := status.NewTxStatusDeriver(params.State, params.LastFullBlockHeight)

	localTxProvider := provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		systemCollection,
		txStatusDeriver,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)
	execNodeTxProvider := provider.NewENTransactionProvider(
		params.Log,
		params.State,
		params.Collections,
		params.ConnFactory,
		params.Communicator,
		params.ExecNodeIdentitiesProvider,
		txStatusDeriver,
		systemCollection,
		sysCollection,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)
	failoverTxProvider := provider.NewFailoverTransactionProvider(localTxProvider, execNodeTxProvider)

	txParams := transactions.Params{
		Log:                          params.Log,
		Metrics:                      params.AccessMetrics,
		State:                        params.State,
		ChainID:                      params.ChainID,
		SystemCollection:             systemCollection,
		StaticCollectionRPCClient:    params.CollectionRPC,
		HistoricalAccessNodeClients:  params.HistoricalAccessNodes,
		NodeCommunicator:             params.Communicator,
		ConnFactory:                  params.ConnFactory,
		NodeProvider:                 params.ExecNodeIdentitiesProvider,
		Blocks:                       params.Blocks,
		Collections:                  params.Collections,
		Transactions:                 params.Transactions,
		TxErrorMessageProvider:       params.TxErrorMessageProvider,
		ScheduledTransactions:        params.ScheduledTransactions,
		TxResultCache:                txResCache,
		TxValidator:                  txValidator,
		TxStatusDeriver:              txStatusDeriver,
		EventsIndex:                  params.EventsIndex,
		TxResultsIndex:               params.TxResultsIndex,
		ScheduledTransactionsEnabled: params.ScheduledCallbacksEnabled,
	}

	switch params.TxResultQueryMode {
	case query_mode.IndexQueryModeLocalOnly:
		txParams.TxProvider = localTxProvider
	case query_mode.IndexQueryModeExecutionNodesOnly:
		txParams.TxProvider = execNodeTxProvider
	case query_mode.IndexQueryModeFailover:
		txParams.TxProvider = failoverTxProvider
	default:
		return nil, fmt.Errorf("invalid tx result query mode: %s", params.TxResultQueryMode)
	}

	txBackend, err := transactions.NewTransactionsBackend(txParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactions backend: %w", err)
	}

	txStreamBackend := txstream.NewTransactionStreamBackend(
		params.Log,
		params.State,
		params.SubscriptionHandler,
		params.BlockTracker,
		txBackend.SendTransaction,
		params.Blocks,
		params.Collections,
		params.Transactions,
		failoverTxProvider,
		txStatusDeriver,
	)

	b := &Backend{
		Accounts:          *accountsBackend,
		Events:            *eventsBackend,
		Scripts:           *scriptsBackend,
		Transactions:      *txBackend,
		TransactionStream: *txStreamBackend,
		backendBlockHeaders: backendBlockHeaders{
			backendBlockBase: backendBlockBase{
				blocks:  params.Blocks,
				headers: params.Headers,
				state:   params.State,
			},
		},
		backendBlockDetails: backendBlockDetails{
			backendBlockBase: backendBlockBase{
				blocks:  params.Blocks,
				headers: params.Headers,
				state:   params.State,
			},
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

		state:               params.State,
		collections:         params.Collections,
		staticCollectionRPC: params.CollectionRPC,
		stateParams:         params.State.Params(),
		versionControl:      params.VersionControl,
		BlockTracker:        params.BlockTracker,
	}

	return b, nil
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
func (b *Backend) GetNodeVersionInfo(_ context.Context) (*accessmodel.NodeVersionInfo, error) {
	sporkID := b.stateParams.SporkID()
	sporkRootBlockHeight := b.stateParams.SporkRootBlockHeight()
	nodeRootBlockHeader := b.stateParams.SealedRoot()
	protocolSnapshot, err := b.state.Final().ProtocolState()
	if err != nil {
		return nil, fmt.Errorf("could not read finalized protocol kvstore: %w", err)
	}

	var compatibleRange *accessmodel.CompatibleRange

	// Version control feature could be disabled
	if b.versionControl != nil {
		compatibleRange = &accessmodel.CompatibleRange{
			StartHeight: b.versionControl.StartHeight(),
			EndHeight:   b.versionControl.EndHeight(),
		}
	}

	nodeInfo := &accessmodel.NodeVersionInfo{
		Semver:               build.Version(),
		Commit:               build.Commit(),
		SporkId:              sporkID,
		ProtocolVersion:      0,
		ProtocolStateVersion: protocolSnapshot.GetProtocolStateVersion(),
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

func (b *Backend) GetNetworkParameters(_ context.Context) accessmodel.NetworkParameters {
	return accessmodel.NetworkParameters{
		ChainID: b.backendNetwork.chainID,
	}
}
