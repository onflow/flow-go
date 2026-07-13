package transactions

import (
	"context"
	"errors"
	"fmt"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

type Transactions struct {
	log     zerolog.Logger
	metrics module.TransactionMetrics

	state   protocol.State
	chainID flow.ChainID

	// RPC Clients & Network
	collectionRPCClient         accessproto.AccessAPIClient // RPC client tied to a fixed collection node
	historicalAccessNodeClients []accessproto.AccessAPIClient
	nodeCommunicator            node_communicator.Communicator
	connectionFactory           connection.ConnectionFactory

	// Storages
	blocks                storage.Blocks
	collections           storage.Collections
	transactions          storage.Transactions
	scheduledTransactions storage.ScheduledTransactionsReader

	txValidator       *validator.TransactionValidator
	txProvider        provider.TransactionProvider
	txStatusDeriver   *txstatus.TxStatusDeriver
	systemCollections *systemcollection.Versioned
	txResultCache     TxResultCache

	scheduledTransactionsEnabled bool
}

var _ access.TransactionsAPI = (*Transactions)(nil)

type Params struct {
	Log                          zerolog.Logger
	Metrics                      module.TransactionMetrics
	State                        protocol.State
	ChainID                      flow.ChainID
	SystemCollections            *systemcollection.Versioned
	StaticCollectionRPCClient    accessproto.AccessAPIClient
	HistoricalAccessNodeClients  []accessproto.AccessAPIClient
	NodeCommunicator             node_communicator.Communicator
	ConnFactory                  connection.ConnectionFactory
	EnableRetries                bool
	NodeProvider                 *rpc.ExecutionNodeIdentitiesProvider
	Blocks                       storage.Blocks
	Collections                  storage.Collections
	Transactions                 storage.Transactions
	ScheduledTransactions        storage.ScheduledTransactionsReader
	TxErrorMessageProvider       error_messages.Provider
	TxResultCache                TxResultCache
	TxProvider                   provider.TransactionProvider
	TxValidator                  *validator.TransactionValidator
	TxStatusDeriver              *txstatus.TxStatusDeriver
	EventsIndex                  *index.EventsIndex
	TxResultsIndex               *index.TransactionResultsIndex
	ScheduledTransactionsEnabled bool
}

func NewTransactionsBackend(params Params) (*Transactions, error) {
	txs := &Transactions{
		log:                          params.Log,
		metrics:                      params.Metrics,
		state:                        params.State,
		chainID:                      params.ChainID,
		systemCollections:            params.SystemCollections,
		collectionRPCClient:          params.StaticCollectionRPCClient,
		historicalAccessNodeClients:  params.HistoricalAccessNodeClients,
		nodeCommunicator:             params.NodeCommunicator,
		connectionFactory:            params.ConnFactory,
		blocks:                       params.Blocks,
		collections:                  params.Collections,
		transactions:                 params.Transactions,
		scheduledTransactions:        params.ScheduledTransactions,
		txResultCache:                params.TxResultCache,
		txValidator:                  params.TxValidator,
		txProvider:                   params.TxProvider,
		txStatusDeriver:              params.TxStatusDeriver,
		scheduledTransactionsEnabled: params.ScheduledTransactionsEnabled,
	}

	return txs, nil
}

// SendTransaction forwards the transaction to the collection node
func (t *Transactions) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	start := time.Now().UTC()

	err := t.txValidator.Validate(ctx, tx)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid transaction: %s", err.Error())
	}

	// send the transaction to the collection node if valid
	err = t.trySendTransaction(ctx, tx)
	if err != nil {
		t.metrics.TransactionSubmissionFailed()
		return rpc.ConvertError(err, "failed to send transaction to a collection node", codes.Internal)
	}

	t.metrics.TransactionReceived(tx.ID(), start)

	// store the transaction locally
	err = t.transactions.Store(tx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to store transaction: %v", err)
	}

	return nil
}

// trySendTransaction tries to transaction to a collection node
func (t *Transactions) trySendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	// if a collection node rpc client was provided at startup, just use that
	if t.collectionRPCClient != nil {
		return t.grpcTxSend(ctx, t.collectionRPCClient, tx)
	}

	// otherwise choose all collection nodes to try
	collNodes, err := t.chooseCollectionNodes(tx.ID())
	if err != nil {
		return fmt.Errorf("failed to determine collection node for tx %x: %w", tx, err)
	}

	// try sending the transaction to one of the chosen collection nodes
	err = t.nodeCommunicator.CallAvailableNode(
		collNodes,
		func(node *flow.IdentitySkeleton) error {
			return t.sendTransactionToCollector(ctx, tx, node.Address)
		},
		nil,
	)

	if err != nil {
		t.log.Info().Err(err).Msg("failed to send transactions to collector nodes")
	}

	return err
}

// chooseCollectionNodes finds a random subset of size sampleSize of collection node addresses from the
// collection node cluster responsible for the given tx
func (t *Transactions) chooseCollectionNodes(txID flow.Identifier) (flow.IdentitySkeletonList, error) {
	// retrieve the set of collector clusters
	currentEpoch, err := t.state.Final().Epochs().Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch: %w", err)
	}
	clusters, err := currentEpoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	targetNodes, ok := clusters.ByTxID(txID)
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %x", txID)
	}

	return targetNodes, nil
}

// sendTransactionToCollection sends the transaction to the given collection node via grpc
func (t *Transactions) sendTransactionToCollector(
	ctx context.Context,
	tx *flow.TransactionBody,
	collectionNodeAddr string,
) error {
	collectionRPC, closer, err := t.connectionFactory.GetCollectionAPIClient(collectionNodeAddr, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to collection node at %s: %w", collectionNodeAddr, err)
	}
	defer closer.Close()

	err = t.grpcTxSend(ctx, collectionRPC, tx)
	if err != nil {
		return fmt.Errorf("failed to send transaction to collection node at %s: %w", collectionNodeAddr, err)
	}
	return nil
}

func (t *Transactions) grpcTxSend(
	ctx context.Context,
	client accessproto.AccessAPIClient,
	tx *flow.TransactionBody,
) error {
	colReq := &accessproto.SendTransactionRequest{
		Transaction: convert.TransactionToMessage(*tx),
	}

	clientDeadline := time.Now().Add(time.Duration(2) * time.Second)
	ctx, cancel := context.WithDeadline(ctx, clientDeadline)
	defer cancel()

	_, err := client.SendTransaction(ctx, colReq)
	return err
}

// SendRawTransaction sends a raw transaction to the collection node
func (t *Transactions) SendRawTransaction(
	ctx context.Context,
	tx *flow.TransactionBody,
) error {
	// send the transaction to the collection node
	return t.trySendTransaction(ctx, tx)
}

func (t *Transactions) GetTransaction(ctx context.Context, txID flow.Identifier) (*flow.TransactionBody, error) {
	tx, err := t.transactions.ByID(txID)
	if err == nil {
		return tx, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return nil, status.Errorf(codes.Internal, "failed to lookup transaction: %v", err)
	}

	// check if it's one of the static system txs
	if tx, ok := t.systemCollections.SearchAll(txID); ok {
		return tx, nil
	}

	// check if it's a scheduled transaction
	if t.scheduledTransactions != nil {
		tx, isScheduledTx, err := t.lookupScheduledTransaction(ctx, txID)
		if err != nil {
			return nil, err
		}
		if isScheduledTx {
			return tx, nil
		}
		// else, this is not a system collection tx. continue with the normal lookup
	}

	// otherwise, check if it's a historic transaction
	return t.getHistoricalTransaction(ctx, txID)
}

// lookupScheduledTransaction looks up the transaction body for a scheduled transaction.
// Returns false and no error if the transaction is not a known scheduled tx.
//
// Expected error returns during normal operation:
//   - [codes.Internal]: if there was an error looking up the events
func (t *Transactions) lookupScheduledTransaction(ctx context.Context, txID flow.Identifier) (*flow.TransactionBody, bool, error) {
	blockID, err := t.scheduledTransactions.BlockIDByTransactionID(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, status.Errorf(codes.Internal, "failed to get scheduled transaction block ID: %v", err)
	}

	header, err := t.state.AtBlockID(blockID).Head()
	if err != nil {
		// since the scheduled transaction is indexed at this block, it must exist in storage, otherwise
		// the node is in an inconsistent state
		err = fmt.Errorf("failed to get block header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, false, err
	}

	scheduledTxs, err := t.txProvider.ScheduledTransactionsByBlockID(ctx, header)
	if err != nil {
		return nil, false, rpc.ConvertError(err, "failed to get scheduled transactions", codes.Internal)
	}

	for _, tx := range scheduledTxs {
		if tx.ID() == txID {
			return tx, true, nil
		}
	}

	// since the scheduled transaction is indexed in this block, it exist in the data generated using
	// events from the block, otherwise the node is in an inconsistent state.
	// TODO: not throwing an irrecoverable here since it's possible that we queried an Execution node
	// for the events, and the EN provided incorrect data. This should be refactored so we handle the
	// condition more precisely.
	return nil, false, status.Errorf(codes.Internal, "scheduled transaction not found, but was indexed in block")
}

func (t *Transactions) GetTransactionsByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) ([]*flow.TransactionBody, error) {
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return t.txProvider.TransactionsByBlockID(ctx, block)
}

func (t *Transactions) GetTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (txResult *accessmodel.TransactionResult, err error) {
	var scriptSize int
	start := time.Now()
	defer func() {
		if err == nil {
			// scriptSize will be 0 for system and scheduled txs. this is OK since the metrics uses
			// size buckets and 0 equates to 1kb, which is about right.
			t.metrics.TransactionResultFetched(time.Since(start), scriptSize)
		}
	}()

	txResult, isSystemTx, err := t.lookupSystemTransactionResult(ctx, txID, blockID, encodingVersion)
	if err != nil {
		return nil, err
	}
	if isSystemTx {
		return txResult, nil
	}

	// if the node is not indexing scheduled transactions, then fallback to the normal lookup. if the
	// request was for a scheduled transaction, it will fail with a not found error.
	if t.scheduledTransactions != nil {
		txResult, isScheduledTx, err := t.lookupScheduledTransactionResult(ctx, txID, blockID, encodingVersion)
		if err != nil {
			return nil, err
		}
		if isScheduledTx {
			return txResult, nil
		}
	}

	txResult, tx, err := t.lookupSubmittedTransactionResult(ctx, txID, blockID, collectionID, encodingVersion)
	if err != nil {
		return nil, err
	}
	if tx != nil {
		scriptSize = len(tx.Script)
	}

	return txResult, nil
}

// lookupSubmittedTransactionResult looks up the transaction result for a user transaction.
// This function assumes that the queried transaction is not a system transaction or scheduled transaction.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the transaction is not found or not in the provided block or collection
//   - [codes.Internal]: if there was an error looking up the block or collection
func (t *Transactions) lookupSubmittedTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, *flow.TransactionBody, error) {
	// 1. lookup the the collection that contains the transaction. if it is not found, then the
	// collection is not yet indexed and the transaction is either unknown or pending.
	//
	// BFT corner case: Only the first finalized collection to contain the transaction is indexed.
	// If the transaction is included in multiple collections in the same or different blocks, the
	// first collection to be _indexed_ by the node is returned. This is not guaranteed to be the
	// first collection in execution order!
	lightCollection, err := t.collections.LightByTransactionID(txID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, status.Errorf(codes.Internal, "failed to find collection for transaction: %v", err)
		}
		// we have already checked if this is a system or scheduled tx. at this point, the tx is either
		// pending, unknown, or from a past spork.
		result, err := t.getUnknownUserTransactionResult(ctx, txID, blockID, collectionID)
		return result, nil, err
	}
	actualCollectionID := lightCollection.ID()
	if collectionID == flow.ZeroID {
		collectionID = actualCollectionID
	} else if collectionID != actualCollectionID {
		return nil, nil, status.Errorf(codes.NotFound, "transaction found in collection %s, but %s was provided", actualCollectionID, collectionID)
	}

	// 2. lookup the block containing the collection.
	block, err := t.blocks.ByCollectionID(collectionID)
	if err != nil {
		// The txID → collectionID index (checked in step 1) and the guaranteeID → blockID index
		// (checked here) are built by separate async components: the collection Indexer and
		// the FinalizedBlockProcessor respectively. During catch-up or under load, the
		// FinalizedBlockProcessor may lag behind, causing ErrNotFound here even though the
		// collection is indexed. This is a transient state that resolves once finalization
		// processing catches up.
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil, status.Errorf(codes.NotFound, "block not found for collection %v", collectionID)
		}

		// any other error is an exception.
		err = fmt.Errorf("failed to find block for collection %v: %w", collectionID, err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}
	actualBlockID := block.ID()
	if blockID == flow.ZeroID {
		blockID = actualBlockID
	} else if blockID != actualBlockID {
		return nil, nil, status.Errorf(codes.NotFound, "transaction found in block %s, but %s was provided", actualBlockID, blockID)
	}

	// 3. lookup the transaction and its result
	tx, err := t.transactions.ByID(txID)
	if err != nil {
		// if we've gotten this far, the transaction must exist in storage otherwise the node is in
		// an inconsistent state
		err = fmt.Errorf("failed to get transaction from storage: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}

	txResult, err := t.txProvider.TransactionResult(ctx, block.ToHeader(), txID, collectionID, encodingVersion)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
		case errors.Is(err, indexer.ErrIndexNotInitialized):
		case errors.Is(err, storage.ErrHeightNotIndexed):
		case status.Code(err) == codes.NotFound:
		default:
			return nil, nil, rpc.ConvertError(err, "failed to retrieve result", codes.Internal)
		}
		// all expected errors fall through to be processed as a known unexecuted transaction.

		// The transaction is not executed yet
		txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Height, false)
		if err != nil {
			irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
			return nil, nil, err
		}

		return &accessmodel.TransactionResult{
			BlockID:       blockID,
			BlockHeight:   block.Height,
			TransactionID: txID,
			Status:        txStatus,
			CollectionID:  collectionID,
		}, tx, nil
	}

	return txResult, tx, nil
}

// lookupSystemTransactionResult looks up the transaction result for a system transaction.
// Returns false and no error if the transaction is not a system tx.
//
// Expected error returns during normal operation:
//   - [codes.InvalidArgument]: if the block ID is not provided
//   - [codes.Internal]: if there was an error looking up the block
func (t *Transactions) lookupSystemTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, bool, error) {
	if _, ok := t.systemCollections.SearchAll(txID); !ok {
		return nil, false, nil // tx is not a system tx
	}

	// block must be provided to get the correct system tx result
	if blockID == flow.ZeroID {
		return nil, false, status.Errorf(codes.InvalidArgument, "block ID is required for system transactions")
	}

	header, err := t.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, false, status.Errorf(codes.NotFound, "could not find block: %v", err)
	}

	result, err := t.txProvider.TransactionResult(ctx, header, txID, flow.ZeroID, encodingVersion)
	return result, true, err
}

// lookupScheduledTransactionResult looks up the transaction result for a scheduled transaction.
// Returns false and no error if the transaction is not a known scheduled tx.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the transaction was found in a different block than the provided block ID
//   - [codes.Internal]: if there was an error looking up the block
func (t *Transactions) lookupScheduledTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, bool, error) {
	scheduledTxBlockID, err := t.scheduledTransactions.BlockIDByTransactionID(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, false, nil // tx is not a scheduled tx
		}
		return nil, false, status.Errorf(codes.Internal, "failed to get scheduled transaction block ID: %v", err)
	}

	if blockID != flow.ZeroID && scheduledTxBlockID != blockID {
		return nil, false, status.Errorf(codes.NotFound, "scheduled transaction found in block %s, but %s was provided", scheduledTxBlockID, blockID)
	}

	header, err := t.state.AtBlockID(scheduledTxBlockID).Head()
	if err != nil {
		// the scheduled transaction is indexed at this block, so this block must exist in storage.
		// otherwise the node is in an inconsistent state
		err = fmt.Errorf("failed to get scheduled transaction's block from storage: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, false, err
	}

	result, err := t.txProvider.TransactionResult(ctx, header, txID, flow.ZeroID, encodingVersion)
	return result, true, err
}

// getUnknownUserTransactionResult returns the transaction result for a transaction that is not yet
// indexed in a finalized block.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the transaction is not found or is in the provided block or collection
//   - [codes.Internal]: if there was an error looking up the transaction, block, or collection.
func (t *Transactions) getUnknownUserTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
) (*accessmodel.TransactionResult, error) {
	tx, err := t.transactions.ByID(txID)
	if err == nil {
		// since the tx was not indexed, if it exists in storage that means it was submitted through
		// this node.
		txStatus, err := t.txStatusDeriver.DeriveUnknownTransactionStatus(tx.ReferenceBlockID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, status.Errorf(codes.NotFound, "transaction's reference block not found")
			}
			err = fmt.Errorf("failed to derive transaction status: %w", err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}

		return &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        txStatus,
		}, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return nil, status.Errorf(codes.Internal, "failed to lookup unknown transaction: %v", err)
	}

	// The transaction does not exist locally, so check if the block or collection help identify its status.
	// If we know the queried block or collection exist locally, then we can avoid querying historical Access Node.
	if blockID != flow.ZeroID {
		_, err := t.blocks.ByID(blockID)
		if err == nil {
			// the user's specified block exists locally, so assume the tx is not yet indexed
			// but will be eventually
			return &accessmodel.TransactionResult{
				TransactionID: txID,
				Status:        flow.TransactionStatusUnknown,
			}, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.Internal, "failed to get block from storage: %v", err)
		}
		// search historical access nodes
	}

	if collectionID != flow.ZeroID {
		_, err := t.collections.LightByID(collectionID)
		if err == nil {
			// the user's specified collection exists locally. since the tx is not indexed, this
			// means the provided collection does not contain the tx
			return nil, status.Errorf(codes.NotFound, "transaction not found in collection %s", collectionID)
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.Internal, "failed to get collection from storage: %v", err)
		}
		// search historical access nodes
	}

	historicalTxResult := t.searchHistoricalAccessNodes(ctx, txID)
	return historicalTxResult, nil
}

func (t *Transactions) GetTransactionResultsByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return t.txProvider.TransactionResultsByBlockID(ctx, block, encodingVersion)
}

// GetTransactionResultByIndex returns transactions Results for an index in a block that is executed,
// pending or finalized transactions  return errors
func (t *Transactions) GetTransactionResultByIndex(
	ctx context.Context,
	blockID flow.Identifier,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	collectionID, err := t.lookupCollectionIDByBlockAndTxIndex(block, index)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "could not find collection for transaction result: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to lookup collection ID in block by index: %v", err)
	}

	return t.txProvider.TransactionResultByIndex(ctx, block, index, collectionID, encodingVersion)
}

// GetSystemTransaction returns a system transaction by ID.
// If no transaction ID is provided, the last system transaction is queried.
// Note: this function only returns privileged system transactions. It does NOT return user scheduled
// transactions, which are also contained within the system collection.
func (t *Transactions) GetSystemTransaction(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
) (*flow.TransactionBody, error) {
	header, err := t.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	if txID == flow.ZeroID {
		systemChunkTx, err := t.systemCollections.
			ByHeight(header.Height).
			SystemChunkTransaction(t.chainID.Chain())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get system chunk transaction: %v", err)
		}
		txID = systemChunkTx.ID()
	}

	tx, ok := t.systemCollections.SearchAll(txID)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "no system transaction with the provided ID found")
	}

	return tx, nil
}

// GetSystemTransactionResult returns a system transaction result by ID.
// If no transaction ID is provided, the last system transaction is queried.
// Note: this function only returns privileged system transactions. It does NOT return user scheduled
// transactions, which are also contained within the system collection.
func (t *Transactions) GetSystemTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	header, err := t.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	if txID == flow.ZeroID {
		systemChunkTx, err := t.systemCollections.
			ByHeight(header.Height).
			SystemChunkTransaction(t.chainID.Chain())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get system chunk transaction: %v", err)
		}
		txID = systemChunkTx.ID()
	}

	txResult, isSystemTx, err := t.lookupSystemTransactionResult(ctx, txID, blockID, encodingVersion)
	if err != nil {
		return nil, err
	}
	if !isSystemTx {
		return nil, status.Errorf(codes.NotFound, "no system transaction with the provided ID found")
	}
	return txResult, nil
}

// GetScheduledTransaction returns the transaction body of the scheduled transaction by ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the scheduled transaction is not found
func (t *Transactions) GetScheduledTransaction(ctx context.Context, scheduledTxID uint64) (*flow.TransactionBody, error) {
	// The scheduled transactions index is only written if execution state indexing is enabled.
	// Note: it's possible indexing is enabled and requests are still served from execution nodes.
	if t.scheduledTransactions == nil {
		return nil, status.Errorf(codes.Unimplemented, "scheduled transactions endpoints require execution state indexing.")
	}

	txID, err := t.scheduledTransactions.TransactionIDByID(scheduledTxID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	tx, isScheduledTx, err := t.lookupScheduledTransaction(ctx, txID)
	if err != nil {
		return nil, err
	}
	if !isScheduledTx {
		// since the scheduled transaction is indexed at this block, it must exist in storage, otherwise
		// the node is in an inconsistent state
		// TODO: not throwing an irrecoverable here since it's possible that we queried an Execution node
		// for the events, and the EN provided incorrect data. This should be refactored so we handle the
		// condition more precisely.
		return nil, status.Errorf(codes.Internal, "scheduled transaction not found, but was indexed in block")
	}
	return tx, nil
}

// GetScheduledTransactionResult returns the transaction result of the scheduled transaction by ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the scheduled transaction is not found
func (t *Transactions) GetScheduledTransactionResult(ctx context.Context, scheduledTxID uint64, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	// The scheduled transactions index is only written if execution state indexing is enabled.
	// Note: it's possible indexing is enabled and requests are still served from execution nodes.
	if t.scheduledTransactions == nil {
		return nil, status.Errorf(codes.Unimplemented, "scheduled transactions endpoints require execution state indexing.")
	}

	txID, err := t.scheduledTransactions.TransactionIDByID(scheduledTxID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	txResult, isScheduledTx, err := t.lookupScheduledTransactionResult(ctx, txID, flow.ZeroID, encodingVersion)
	if err != nil {
		return nil, err
	}
	if !isScheduledTx {
		// since the scheduled transaction is indexed at this block, it must exist in storage, otherwise
		// the node is in an inconsistent state
		// TODO: not throwing an irrecoverable here since it's possible that we queried an Execution node
		// for the events, and the EN provided incorrect data. This should be refactored so we handle the
		// condition more precisely.
		return nil, status.Errorf(codes.Internal, "scheduled transaction not found, but was indexed in block")
	}
	return txResult, nil
}

// getHistoricalTransaction searches the historical access nodes for the transaction body.
//
// All errors are benign and side-effect free for the node. They indicate an issue communicating with
// external nodes.
func (t *Transactions) getHistoricalTransaction(
	ctx context.Context,
	txID flow.Identifier,
) (*flow.TransactionBody, error) {
	for _, historicalNode := range t.historicalAccessNodeClients {
		txResp, err := historicalNode.GetTransaction(ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
		if err == nil {
			tx, err := convert.MessageToTransaction(txResp.Transaction, t.chainID.Chain())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "could not convert transaction: %v", err)
			}

			// Found on a historical node. Report
			return &tx, nil
		}
		// Otherwise, if not found, just continue
		if status.Code(err) == codes.NotFound {
			continue
		}
		// TODO should we do something if the error isn't not found?
	}
	return nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", txID)
}

// searchHistoricalAccessNodes searches the historical access nodes for the transaction result
// and caches the result if enabled.
func (t *Transactions) searchHistoricalAccessNodes(
	ctx context.Context,
	txID flow.Identifier,
) *accessmodel.TransactionResult {
	// if the tx is not known locally, search the historical access nodes
	if result, ok := t.txResultCache.Get(txID); ok {
		return result
	}

	historicalTxResult, err := t.getHistoricalTransactionResult(ctx, txID)
	if err != nil {
		// if tx not found on historic access nodes either, then assume that the tx was
		// submitted to a different AN and return status as unknown
		historicalTxResult = &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        flow.TransactionStatusUnknown,
		}
	}

	// always cache the result even if it's an error to avoid unnecessary load on the nodes.
	// the cache is limited so retries will happen eventually. users can also query the nodes
	// directly for more precise results.
	t.txResultCache.Add(txID, historicalTxResult)

	return historicalTxResult
}

// getHistoricalTransactionResult searches the historical access nodes for the transaction result.
//
// All errors are benign and side-effect free for the node. They indicate an issue communicating with
// external nodes.
func (t *Transactions) getHistoricalTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (*accessmodel.TransactionResult, error) {
	for _, historicalNode := range t.historicalAccessNodeClients {
		result, err := historicalNode.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
		if err == nil {
			// Found on a historical node. Report
			if result.GetStatus() == entities.TransactionStatus_UNKNOWN {
				// We've moved to returning Status UNKNOWN instead of an error with the NotFound status,
				// Therefore we should continue and look at the next access node for answers.
				continue
			}

			if result.GetStatus() == entities.TransactionStatus_PENDING {
				// This is on a historical node. No transactions from it will ever be
				// executed, therefore we should consider this expired
				result.Status = entities.TransactionStatus_EXPIRED
			}

			txResult, err := convert.MessageToTransactionResult(result)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "could not convert transaction result: %v", err)
			}

			return txResult, nil
		}
		// Otherwise, if not found, just continue
		if status.Code(err) == codes.NotFound {
			continue
		}
		// TODO should we do something if the error isn't not found?
	}
	return nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", txID)
}

// lookupCollectionIDByBlockAndTxIndex returns the collection ID that contains the transasction with
// the provided transaction index.
//
// If the index is larger that the number of user transactions, flow.ZeroID is returned, indicating
// that the transaction is a system transaction. The caller should verify that the index does in fact
// correspond to a system transaction.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] - if any of the collections in the block cannot be found.
func (t *Transactions) lookupCollectionIDByBlockAndTxIndex(block *flow.Block, index uint32) (flow.Identifier, error) {
	txIndex := uint32(0)
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find collection %s: %w", guarantee.CollectionID, err)
		}

		for range collection.Transactions {
			if txIndex == index {
				return guarantee.CollectionID, nil
			}
			txIndex++
		}
	}

	// otherwise, assume it's a system transaction and return the ZeroID
	return flow.ZeroID, nil
}
