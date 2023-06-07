package backend

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	lru "github.com/hashicorp/golang-lru"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
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

// DefaultSnapshotHistoryLimit the amount of blocks to look back in state
// when recursively searching for a valid snapshot
const DefaultSnapshotHistoryLimit = 50

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
	connFactory       ConnectionFactory
	connSelector      ConnectionSelector
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
	accessMetrics module.AccessMetrics,
	connFactory ConnectionFactory,
	connSelector ConnectionSelector,
	retryEnabled bool,
	maxHeightRange uint,
	preferredExecutionNodeIDs []string,
	fixedExecutionNodeIDs []string,
	log zerolog.Logger,
	snapshotHistoryLimit int,
	archiveAddressList []string,
) *Backend {
	retry := newRetry()
	if retryEnabled {
		retry.Activate()
	}

	loggedScripts, err := lru.New(DefaultLoggedScriptsCacheSize)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize script logging cache")
	}

	b := &Backend{
		state: state,
		// create the sub-backends
		backendScripts: backendScripts{
			headers:            headers,
			executionReceipts:  executionReceipts,
			connFactory:        connFactory,
			connSelector:       connSelector,
			state:              state,
			log:                log,
			metrics:            accessMetrics,
			loggedScripts:      loggedScripts,
			archiveAddressList: archiveAddressList,
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
			transactionMetrics:   accessMetrics,
			retry:                retry,
			connFactory:          connFactory,
			connSelector:         connSelector,
			previousAccessNodes:  historicalAccessNodes,
			log:                  log,
		},
		backendEvents: backendEvents{
			state:             state,
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			connSelector:      connSelector,
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
			connSelector:      connSelector,
			log:               log,
		},
		backendExecutionResults: backendExecutionResults{
			executionResults: executionResults,
		},
		backendNetwork: backendNetwork{
			state:                state,
			chainID:              chainID,
			snapshotHistoryLimit: snapshotHistoryLimit,
		},
		collections:       collections,
		executionReceipts: executionReceipts,
		connFactory:       connFactory,
		connSelector:      connSelector,
		chainID:           chainID,
	}

	retry.SetBackend(b)

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
func (b *Backend) GetNodeVersionInfo(ctx context.Context) (*access.NodeVersionInfo, error) {
	stateParams := b.state.Params()
	sporkId, err := stateParams.SporkID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read spork ID: %v", err)
	}

	protocolVersion, err := stateParams.ProtocolVersion()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read protocol version: %v", err)
	}

	return &access.NodeVersionInfo{
		Semver:          build.Semver(),
		Commit:          build.Commit(),
		SporkId:         sporkId,
		ProtocolVersion: uint64(protocolVersion),
	}, nil
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

// GetLatestProtocolStateSnapshot returns the latest finalized snapshot
func (b *Backend) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	snapshot := b.state.Final()

	validSnapshot, err := b.getValidSnapshot(snapshot, 0)
	if err != nil {
		return nil, err
	}

	return convert.SnapshotToBytes(validSnapshot)
}
