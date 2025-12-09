package transactions

import (
	"context"
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
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
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/retrier"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
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

	systemTxID flow.Identifier

	// RPC Clients & Network
	collectionRPCClient         accessproto.AccessAPIClient // RPC client tied to a fixed collection node
	historicalAccessNodeClients []accessproto.AccessAPIClient
	nodeCommunicator            node_communicator.Communicator
	connectionFactory           connection.ConnectionFactory
	retrier                     retrier.Retrier

	// Storages
	blocks       storage.Blocks
	collections  storage.Collections
	transactions storage.Transactions
	events       storage.Events

	txResultCache *lru.Cache[flow.Identifier, *accessmodel.TransactionResult]

	txValidator     *validator.TransactionValidator
	txProvider      provider.TransactionProvider
	txStatusDeriver *txstatus.TxStatusDeriver

	scheduledCallbacksEnabled bool
}

var _ access.TransactionsAPI = (*Transactions)(nil)

type Params struct {
	Log                         zerolog.Logger
	Metrics                     module.TransactionMetrics
	State                       protocol.State
	ChainID                     flow.ChainID
	SystemTxID                  flow.Identifier
	StaticCollectionRPCClient   accessproto.AccessAPIClient
	HistoricalAccessNodeClients []accessproto.AccessAPIClient
	NodeCommunicator            node_communicator.Communicator
	ConnFactory                 connection.ConnectionFactory
	EnableRetries               bool
	NodeProvider                *rpc.ExecutionNodeIdentitiesProvider
	Blocks                      storage.Blocks
	Collections                 storage.Collections
	Transactions                storage.Transactions
	Events                      storage.Events
	TxErrorMessageProvider      error_messages.Provider
	TxResultCache               *lru.Cache[flow.Identifier, *accessmodel.TransactionResult]
	TxProvider                  provider.TransactionProvider
	TxValidator                 *validator.TransactionValidator
	TxStatusDeriver             *txstatus.TxStatusDeriver
	EventsIndex                 *index.EventsIndex
	TxResultsIndex              *index.TransactionResultsIndex
	ScheduledCallbacksEnabled   bool
}

func NewTransactionsBackend(params Params) (*Transactions, error) {
	txs := &Transactions{
		log:                         params.Log,
		metrics:                     params.Metrics,
		state:                       params.State,
		chainID:                     params.ChainID,
		systemTxID:                  params.SystemTxID,
		collectionRPCClient:         params.StaticCollectionRPCClient,
		historicalAccessNodeClients: params.HistoricalAccessNodeClients,
		nodeCommunicator:            params.NodeCommunicator,
		connectionFactory:           params.ConnFactory,
		blocks:                      params.Blocks,
		collections:                 params.Collections,
		transactions:                params.Transactions,
		events:                      params.Events,
		txResultCache:               params.TxResultCache,
		txValidator:                 params.TxValidator,
		txProvider:                  params.TxProvider,
		txStatusDeriver:             params.TxStatusDeriver,
		scheduledCallbacksEnabled:   params.ScheduledCallbacksEnabled,
	}

	if params.EnableRetries {
		txs.retrier = retrier.NewRetrier(
			params.Log,
			params.Blocks,
			params.Collections,
			txs,
			params.TxStatusDeriver,
		)
	} else {
		txs.retrier = retrier.NewNoopRetrier()
	}

	return txs, nil
}

// SendTransaction forwards the transaction to the collection node
func (t *Transactions) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	now := time.Now().UTC()

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

	t.metrics.TransactionReceived(tx.ID(), now)

	// store the transaction locally
	err = t.transactions.Store(tx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to store transaction: %v", err)
	}

	go t.registerTransactionForRetry(tx)

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

	var sendError error
	logAnyError := func() {
		if sendError != nil {
			t.log.Info().Err(err).Msg("failed to send transactions to collector nodes")
		}
	}
	defer logAnyError()

	// try sending the transaction to one of the chosen collection nodes
	sendError = t.nodeCommunicator.CallAvailableNode(
		collNodes,
		func(node *flow.IdentitySkeleton) error {
			err = t.sendTransactionToCollector(ctx, tx, node.Address)
			if err != nil {
				return err
			}
			return nil
		},
		nil,
	)

	return sendError
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
	// look up transaction from storage
	tx, err := t.transactions.ByID(txID)
	txErr := rpc.ConvertStorageError(err)

	if txErr != nil {
		if status.Code(txErr) == codes.NotFound {
			return t.getHistoricalTransaction(ctx, txID)
		}
		// Other Error trying to retrieve the transaction, return with err
		return nil, txErr
	}

	return tx, nil
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
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	// look up transaction from storage
	start := time.Now()

	tx, err := t.transactions.ByID(txID)
	if err != nil {
		txErr := rpc.ConvertStorageError(err)
		if status.Code(txErr) != codes.NotFound {
			return nil, txErr
		}

		// Tx not found. If we have historical Sporks setup, lets look through those as well
		if t.txResultCache != nil {
			val, ok := t.txResultCache.Get(txID)
			if ok {
				return val, nil
			}
		}
		historicalTxResult, err := t.getHistoricalTransactionResult(ctx, txID)
		if err != nil {
			// if tx not found in old access nodes either, then assume that the tx was submitted to a different AN
			// and return status as unknown
			txStatus := flow.TransactionStatusUnknown
			result := &accessmodel.TransactionResult{
				Status:     txStatus,
				StatusCode: uint(txStatus),
			}
			if t.txResultCache != nil {
				t.txResultCache.Add(txID, result)
			}
			return result, nil
		}

		if t.txResultCache != nil {
			t.txResultCache.Add(txID, historicalTxResult)
		}
		return historicalTxResult, nil
	}

	block, err := t.retrieveBlock(blockID, collectionID, txID)
	// an error occurred looking up the block or the requested block or collection was not found.
	// If looking up the block based solely on the txID returns not found, then no error is
	// returned since the block may not be finalized yet.
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	var blockHeight uint64
	var txResult *accessmodel.TransactionResult
	// access node may not have the block if it hasn't yet been finalized, hence block can be nil at this point
	if block != nil {
		txResult, err = t.lookupTransactionResult(ctx, txID, block.ToHeader(), requiredEventEncodingVersion)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to retrieve result", codes.Internal)
		}

		// an additional check to ensure the correctness of the collection ID.
		expectedCollectionID, err := t.lookupCollectionIDInBlock(block, txID)
		if err != nil {
			// if the collection has not been indexed yet, the lookup will return a not found error.
			// if the request included a blockID or collectionID in its the search criteria, not found
			// should result in an error because it's not possible to guarantee that the result found
			// is the correct one.
			if blockID != flow.ZeroID || collectionID != flow.ZeroID {
				return nil, rpc.ConvertStorageError(err)
			}
		}

		if collectionID == flow.ZeroID {
			collectionID = expectedCollectionID
		} else if collectionID != expectedCollectionID {
			return nil, status.Error(codes.InvalidArgument, "transaction not found in provided collection")
		}

		blockID = block.ID()
		blockHeight = block.Height
	}

	// If there is still no transaction result, provide one based on available information.
	if txResult == nil {
		var txStatus flow.TransactionStatus
		// Derive the status of the transaction.
		if block == nil {
			txStatus, err = t.txStatusDeriver.DeriveUnknownTransactionStatus(tx.ReferenceBlockID)
		} else {
			txStatus, err = t.txStatusDeriver.DeriveFinalizedTransactionStatus(blockHeight, false)
		}

		if err != nil {
			irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
			return nil, err
		}

		txResult = &accessmodel.TransactionResult{
			BlockID:       blockID,
			BlockHeight:   blockHeight,
			TransactionID: txID,
			Status:        txStatus,
			CollectionID:  collectionID,
		}
	} else {
		txResult.CollectionID = collectionID
	}

	t.metrics.TransactionResultFetched(time.Since(start), len(tx.Script))

	return txResult, nil
}

// lookupCollectionIDInBlock returns the collection ID based on the transaction ID. The lookup is performed in block
// collections.
func (t *Transactions) lookupCollectionIDInBlock(
	block *flow.Block,
	txID flow.Identifier,
) (flow.Identifier, error) {
	for _, guarantee := range block.Payload.Guarantees {
		collectionID := guarantee.CollectionID
		collection, err := t.collections.LightByID(collectionID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("failed to get collection %s in indexed block: %w", collectionID, err)
		}
		for _, collectionTxID := range collection.Transactions {
			if collectionTxID == txID {
				return collectionID, nil
			}
		}
	}
	return flow.ZeroID, ErrTransactionNotInBlock
}

// retrieveBlock function returns a block based on the input arguments.
// The block ID lookup has the highest priority, followed by the collection ID lookup.
// If both are missing, the default lookup by transaction ID is performed.
//
// If looking up the block based solely on the txID returns not found, then no error is returned.
//
// Expected errors:
// - storage.ErrNotFound if the requested block or collection was not found.
func (t *Transactions) retrieveBlock(
	blockID flow.Identifier,
	collectionID flow.Identifier,
	txID flow.Identifier,
) (*flow.Block, error) {
	if blockID != flow.ZeroID {
		return t.blocks.ByID(blockID)
	}

	if collectionID != flow.ZeroID {
		return t.blocks.ByCollectionID(collectionID)
	}

	// find the block for the transaction
	block, err := t.lookupBlock(txID)

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	return block, nil
}

func (t *Transactions) GetTransactionResultsByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return t.txProvider.TransactionResultsByBlockID(ctx, block, requiredEventEncodingVersion)
}

// GetTransactionResultByIndex returns transactions Results for an index in a block that is executed,
// pending or finalized transactions  return errors
func (t *Transactions) GetTransactionResultByIndex(
	ctx context.Context,
	blockID flow.Identifier,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return t.txProvider.TransactionResultByIndex(ctx, block, index, requiredEventEncodingVersion)
}

// GetSystemTransaction returns system transaction
func (t *Transactions) GetSystemTransaction(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
) (*flow.TransactionBody, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	if txID == flow.ZeroID {
		txID = t.systemTxID
	}

	return t.txProvider.SystemTransaction(ctx, block, txID)
}

// GetSystemTransactionResult returns system transaction result
func (t *Transactions) GetSystemTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	if txID == flow.ZeroID {
		txID = t.systemTxID
	}

	return t.txProvider.SystemTransactionResult(ctx, block, txID, requiredEventEncodingVersion)
}

// Error returns:
//   - `storage.ErrNotFound` - collection referenced by transaction or block by a collection has not been found.
//   - all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
func (t *Transactions) lookupBlock(txID flow.Identifier) (*flow.Block, error) {
	collection, err := t.collections.LightByTransactionID(txID)
	if err != nil {
		return nil, err
	}

	block, err := t.blocks.ByCollectionID(collection.ID())
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (t *Transactions) lookupTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	header *flow.Header,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	txResult, err := t.txProvider.TransactionResult(ctx, header, txID, requiredEventEncodingVersion, optimistic_sync.Criteria{})
	if err != nil {
		// if either the storage or execution node reported no results or there were not enough execution results
		if status.Code(err) == codes.NotFound {
			// No result yet, indicate that it has not been executed
			return nil, nil
		}
		// Other Error trying to retrieve the result, return with err
		return nil, err
	}

	// considered executed as long as some result is returned, even if it's an error message
	return txResult, nil
}

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

func (t *Transactions) registerTransactionForRetry(tx *flow.TransactionBody) {
	referenceBlock, err := t.state.AtBlockID(tx.ReferenceBlockID).Head()
	if err != nil {
		return
	}

	t.retrier.RegisterTransaction(referenceBlock.Height, tx)
}

// ATTENTION: might be a source of problems in future. We run this code on finalization gorotuine,
// potentially lagging finalization events if operations take long time.
// We might need to move this logic on dedicated goroutine and provide a way to skip finalization events if they are delivered
// too often for this engine. An example of similar approach - https://github.com/onflow/flow-go/blob/10b0fcbf7e2031674c00f3cdd280f27bd1b16c47/engine/common/follower/compliance_engine.go#L201..
// No errors expected during normal operations.
func (t *Transactions) ProcessFinalizedBlockHeight(height uint64) error {
	return t.retrier.Retry(height)
}
