package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
)

type backendTransactions struct {
	TransactionsLocalDataProvider
	staticCollectionRPC  accessproto.AccessAPIClient // rpc client tied to a fixed collection node
	transactions         storage.Transactions
	executionReceipts    storage.ExecutionReceipts
	chainID              flow.ChainID
	transactionMetrics   module.TransactionMetrics
	transactionValidator *access.TransactionValidator
	retry                *Retry
	connFactory          connection.ConnectionFactory

	previousAccessNodes  []accessproto.AccessAPIClient
	log                  zerolog.Logger
	nodeCommunicator     Communicator
	txResultCache        *lru.Cache[flow.Identifier, *access.TransactionResult]
	txErrorMessagesCache *lru.Cache[flow.Identifier, string] // cache for transactions error messages, indexed by hash(block_id, tx_id).
	txResultQueryMode    IndexQueryMode
}

// SendTransaction forwards the transaction to the collection node
func (b *backendTransactions) SendTransaction(
	ctx context.Context,
	tx *flow.TransactionBody,
) error {
	now := time.Now().UTC()

	err := b.transactionValidator.Validate(tx)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid transaction: %s", err.Error())
	}

	// send the transaction to the collection node if valid
	err = b.trySendTransaction(ctx, tx)
	if err != nil {
		b.transactionMetrics.TransactionSubmissionFailed()
		return rpc.ConvertError(err, "failed to send transaction to a collection node", codes.Internal)
	}

	b.transactionMetrics.TransactionReceived(tx.ID(), now)

	// store the transaction locally
	err = b.transactions.Store(tx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to store transaction: %v", err)
	}

	if b.retry.IsActive() {
		go b.registerTransactionForRetry(tx)
	}

	return nil
}

// trySendTransaction tries to transaction to a collection node
func (b *backendTransactions) trySendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	// if a collection node rpc client was provided at startup, just use that
	if b.staticCollectionRPC != nil {
		return b.grpcTxSend(ctx, b.staticCollectionRPC, tx)
	}

	// otherwise choose all collection nodes to try
	collNodes, err := b.chooseCollectionNodes(tx.ID())
	if err != nil {
		return fmt.Errorf("failed to determine collection node for tx %x: %w", tx, err)
	}

	var sendError error
	logAnyError := func() {
		if sendError != nil {
			b.log.Info().Err(err).Msg("failed to send transactions  to collector nodes")
		}
	}
	defer logAnyError()

	// try sending the transaction to one of the chosen collection nodes
	sendError = b.nodeCommunicator.CallAvailableNode(
		collNodes,
		func(node *flow.Identity) error {
			err = b.sendTransactionToCollector(ctx, tx, node.Address)
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
func (b *backendTransactions) chooseCollectionNodes(txID flow.Identifier) (flow.IdentityList, error) {
	// retrieve the set of collector clusters
	clusters, err := b.state.Final().Epochs().Current().Clustering()
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
func (b *backendTransactions) sendTransactionToCollector(
	ctx context.Context,
	tx *flow.TransactionBody,
	collectionNodeAddr string,
) error {
	collectionRPC, closer, err := b.connFactory.GetAccessAPIClient(collectionNodeAddr, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to collection node at %s: %w", collectionNodeAddr, err)
	}
	defer closer.Close()

	err = b.grpcTxSend(ctx, collectionRPC, tx)
	if err != nil {
		return fmt.Errorf("failed to send transaction to collection node at %s: %w", collectionNodeAddr, err)
	}
	return nil
}

func (b *backendTransactions) grpcTxSend(ctx context.Context, client accessproto.AccessAPIClient, tx *flow.TransactionBody) error {
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
func (b *backendTransactions) SendRawTransaction(
	ctx context.Context,
	tx *flow.TransactionBody,
) error {
	// send the transaction to the collection node
	return b.trySendTransaction(ctx, tx)
}

func (b *backendTransactions) GetTransaction(ctx context.Context, txID flow.Identifier) (*flow.TransactionBody, error) {
	// look up transaction from storage
	tx, err := b.transactions.ByID(txID)
	txErr := rpc.ConvertStorageError(err)

	if txErr != nil {
		if status.Code(txErr) == codes.NotFound {
			return b.getHistoricalTransaction(ctx, txID)
		}
		// Other Error trying to retrieve the transaction, return with err
		return nil, txErr
	}

	return tx, nil
}

func (b *backendTransactions) GetTransactionsByBlockID(
	_ context.Context,
	blockID flow.Identifier,
) ([]*flow.TransactionBody, error) {
	var transactions []*flow.TransactionBody

	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := b.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	systemTx, err := blueprints.SystemChunkTransaction(b.chainID.Chain())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get system chunk transaction: %v", err)
	}

	transactions = append(transactions, systemTx)

	return transactions, nil
}

func (b *backendTransactions) GetTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	// look up transaction from storage
	start := time.Now()

	tx, err := b.transactions.ByID(txID)
	if err != nil {
		txErr := rpc.ConvertStorageError(err)

		if status.Code(txErr) != codes.NotFound {
			return nil, txErr
		}

		// Tx not found. If we have historical Sporks setup, lets look through those as well
		if b.txResultCache != nil {
			val, ok := b.txResultCache.Get(txID)
			if ok {
				return val, nil
			}
		}
		historicalTxResult, err := b.getHistoricalTransactionResult(ctx, txID)
		if err != nil {
			// if tx not found in old access nodes either, then assume that the tx was submitted to a different AN
			// and return status as unknown
			txStatus := flow.TransactionStatusUnknown
			result := &access.TransactionResult{
				Status:     txStatus,
				StatusCode: uint(txStatus),
			}
			if b.txResultCache != nil {
				b.txResultCache.Add(txID, result)
			}
			return result, nil
		}

		if b.txResultCache != nil {
			b.txResultCache.Add(txID, historicalTxResult)
		}
		return historicalTxResult, nil
	}

	block, err := b.retrieveBlock(blockID, collectionID, txID)
	// an error occurred looking up the block or the requested block or collection was not found.
	// If looking up the block based solely on the txID returns not found, then no error is
	// returned since the block may not be finalized yet.
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	var blockHeight uint64
	var txResult *access.TransactionResult
	// access node may not have the block if it hasn't yet been finalized, hence block can be nil at this point
	if block != nil {
		txResult, err = b.lookupTransactionResult(ctx, txID, block, requiredEventEncodingVersion)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
		}

		// an additional check to ensure the correctness of the collection ID.
		expectedCollectionID, err := b.lookupCollectionIDInBlock(block, txID)
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
		blockHeight = block.Header.Height
	}

	// If there is still no transaction result, provide one based on available information.
	if txResult == nil {
		// Derive the status of the transaction.
		txStatus, err := b.deriveTransactionStatus(tx, false, block)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		txResult = &access.TransactionResult{
			BlockID:       blockID,
			BlockHeight:   blockHeight,
			TransactionID: txID,
			Status:        txStatus,
			CollectionID:  collectionID,
		}
	} else {
		txResult.CollectionID = collectionID
	}

	b.transactionMetrics.TransactionResultFetched(time.Since(start), len(tx.Script))

	return txResult, nil
}

// retrieveBlock function returns a block based on the input argument. The block ID lookup has the highest priority,
// followed by the collection ID lookup. If both are missing, the default lookup by transaction ID is performed.
func (b *backendTransactions) retrieveBlock(
	// the requested block or collection was not found. If looking up the block based solely on the txID returns
	// not found, then no error is returned.
	blockID flow.Identifier,
	collectionID flow.Identifier,
	txID flow.Identifier,
) (*flow.Block, error) {
	if blockID != flow.ZeroID {
		return b.blocks.ByID(blockID)
	}

	if collectionID != flow.ZeroID {
		return b.blocks.ByCollectionID(collectionID)
	}

	// find the block for the transaction
	block, err := b.lookupBlock(txID)

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	return block, nil
}

func (b *backendTransactions) GetTransactionResultsByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*access.TransactionResult, error) {
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	switch b.txResultQueryMode {
	case IndexQueryModeExecutionNodesOnly:
		return b.getTransactionResultsByBlockIDFromExecutionNode(ctx, block, requiredEventEncodingVersion)
	case IndexQueryModeLocalOnly:
		return b.GetTransactionResultsByBlockIDFromStorage(ctx, block, requiredEventEncodingVersion)
	case IndexQueryModeFailover:
		results, err := b.GetTransactionResultsByBlockIDFromStorage(ctx, block, requiredEventEncodingVersion)
		if err == nil {
			return results, nil
		}

		if err != nil {
			return nil, err
		}

		return b.getTransactionResultsByBlockIDFromExecutionNode(ctx, block, requiredEventEncodingVersion)
	default:
		return nil, status.Errorf(codes.Internal, "unknown transaction result query mode: %v", b.txResultQueryMode)
	}
}

func (b *backendTransactions) getTransactionResultsByBlockIDFromExecutionNode(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*access.TransactionResult, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		if IsInsufficientExecutionReceipts(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	resp, err := b.getTransactionResultsByBlockIDFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	results := make([]*access.TransactionResult, 0, len(resp.TransactionResults))
	i := 0
	errInsufficientResults := status.Errorf(
		codes.Internal,
		"number of transaction results returned by execution node is less than the number of transactions  in the block",
	)

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := b.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		for _, txID := range collection.Transactions {
			// bounds check. this means the EN returned fewer transaction results than the transactions  in the block
			if i >= len(resp.TransactionResults) {
				return nil, errInsufficientResults
			}
			txResult := resp.TransactionResults[i]

			// tx body is irrelevant to status if it's in an executed block
			txStatus, err := b.deriveTransactionStatus(nil, true, block)
			if err != nil {
				if !errors.Is(err, state.ErrUnknownSnapshotReference) {
					irrecoverable.Throw(ctx, err)
				}
				return nil, rpc.ConvertStorageError(err)
			}
			events, err := convert.MessagesToEventsWithEncodingConversion(txResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
			if err != nil {
				return nil, status.Errorf(codes.Internal,
					"failed to convert events to message in txID %x: %v", txID, err)
			}

			results = append(results, &access.TransactionResult{
				Status:        txStatus,
				StatusCode:    uint(txResult.GetStatusCode()),
				Events:        events,
				ErrorMessage:  txResult.GetErrorMessage(),
				BlockID:       blockID,
				TransactionID: txID,
				CollectionID:  guarantee.CollectionID,
				BlockHeight:   block.Header.Height,
			})

			i++
		}
	}

	// after iterating through all transactions  in each collection, i equals the total number of
	// user transactions  in the block
	txCount := i

	sporkRootBlockHeight, err := b.state.Params().SporkRootBlockHeight()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve root block: %v", err)
	}

	// root block has no system transaction result
	if block.Header.Height > sporkRootBlockHeight {
		// system chunk transaction

		// resp.TransactionResults includes the system tx result, so there should be exactly one
		// more result than txCount
		if txCount != len(resp.TransactionResults)-1 {
			if txCount >= len(resp.TransactionResults) {
				return nil, errInsufficientResults
			}
			// otherwise there are extra results
			// TODO(bft): slashable offense
			return nil, status.Errorf(codes.Internal, "number of transaction results returned by execution node is more than the number of transactions  in the block")
		}

		systemTx, err := blueprints.SystemChunkTransaction(b.chainID.Chain())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not get system chunk transaction: %v", err)
		}
		systemTxResult := resp.TransactionResults[len(resp.TransactionResults)-1]
		systemTxStatus, err := b.deriveTransactionStatus(systemTx, true, block)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert events from system tx result", codes.Internal)
		}

		results = append(results, &access.TransactionResult{
			Status:        systemTxStatus,
			StatusCode:    uint(systemTxResult.GetStatusCode()),
			Events:        events,
			ErrorMessage:  systemTxResult.GetErrorMessage(),
			BlockID:       blockID,
			TransactionID: systemTx.ID(),
			BlockHeight:   block.Header.Height,
		})
	}
	return results, nil
}

// GetTransactionResultByIndex returns transactions Results for an index in a block that is executed,
// pending or finalized transactions  return errors
func (b *backendTransactions) GetTransactionResultByIndex(
	ctx context.Context,
	blockID flow.Identifier,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	// TODO: https://github.com/onflow/flow-go/issues/2175 so caching doesn't cause a circular dependency
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	switch b.txResultQueryMode {
	case IndexQueryModeExecutionNodesOnly:
		return b.getTransactionResultByIndexFromExecutionNode(ctx, block, index, requiredEventEncodingVersion)
	case IndexQueryModeLocalOnly:
		return b.GetTransactionResultByIndexFromStorage(ctx, block, index, requiredEventEncodingVersion)
	case IndexQueryModeFailover:
		result, err := b.GetTransactionResultByIndexFromStorage(ctx, block, index, requiredEventEncodingVersion)
		if err == nil {
			return result, nil
		}

		if err != nil {
			// Skip error type NotFound, to request transaction result from EN
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}
		}

		result, err = b.getTransactionResultByIndexFromExecutionNode(ctx, block, index, requiredEventEncodingVersion)
		if err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, status.Errorf(codes.Internal, "unknown transaction result query mode: %v", b.txResultQueryMode)
	}
}

func (b *backendTransactions) getTransactionResultByIndexFromExecutionNode(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	blockID := block.ID()
	// create request and forward to EN
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		if IsInsufficientExecutionReceipts(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	resp, err := b.getTransactionResultByIndexFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := b.deriveTransactionStatus(nil, true, block)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert events in blockID %x: %v", blockID, err)
	}

	// convert to response, cache and return
	return &access.TransactionResult{
		Status:       txStatus,
		StatusCode:   uint(resp.GetStatusCode()),
		Events:       events,
		ErrorMessage: resp.GetErrorMessage(),
		BlockID:      blockID,
		BlockHeight:  block.Header.Height,
	}, nil
}

// GetSystemTransaction returns system transaction
func (b *backendTransactions) GetSystemTransaction(ctx context.Context, _ flow.Identifier) (*flow.TransactionBody, error) {
	systemTx, err := blueprints.SystemChunkTransaction(b.chainID.Chain())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get system chunk transaction: %v", err)
	}

	return systemTx, nil
}

// GetSystemTransactionResult returns system transaction result
func (b *backendTransactions) GetSystemTransactionResult(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*access.TransactionResult, error) {
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	req := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}
	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		if IsInsufficientExecutionReceipts(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	resp, err := b.getTransactionResultsByBlockIDFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	systemTx, err := blueprints.SystemChunkTransaction(b.chainID.Chain())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get system chunk transaction: %v", err)
	}

	systemTxResult := resp.TransactionResults[len(resp.TransactionResults)-1]
	systemTxStatus, err := b.deriveTransactionStatus(systemTx, true, block)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to convert events from system tx result", codes.Internal)
	}

	return &access.TransactionResult{
		Status:        systemTxStatus,
		StatusCode:    uint(systemTxResult.GetStatusCode()),
		Events:        events,
		ErrorMessage:  systemTxResult.GetErrorMessage(),
		BlockID:       blockID,
		TransactionID: systemTx.ID(),
		BlockHeight:   block.Header.Height,
	}, nil
}

// Error returns:
//   - `storage.ErrNotFound` - collection referenced by transaction or block by a collection has not been found.
//   - all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
func (b *backendTransactions) lookupBlock(txID flow.Identifier) (*flow.Block, error) {
	collection, err := b.collections.LightByTransactionID(txID)
	if err != nil {
		return nil, err
	}

	block, err := b.blocks.ByCollectionID(collection.ID())
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (b *backendTransactions) lookupTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	switch b.txResultQueryMode {
	case IndexQueryModeExecutionNodesOnly:
		txResult, err := b.getTransactionResultFromExecutionNode(ctx, block, txID, requiredEventEncodingVersion)
		if err != nil {
			// if either the execution node reported no results or there were not enough execution results
			if status.Code(err) == codes.NotFound {
				// No result yet, indicate that it has not been executed
				return nil, nil
			}
			// Other Error trying to retrieve the result, return with err
			return nil, err
		}

		// considered executed as long as some result is returned, even if it's an error message
		return txResult, nil
	case IndexQueryModeLocalOnly:
		return b.GetTransactionResultFromStorage(ctx, block, txID, requiredEventEncodingVersion)
	case IndexQueryModeFailover:
		txResult, err := b.GetTransactionResultFromStorage(ctx, block, txID, requiredEventEncodingVersion)
		if err == nil {
			return txResult, nil
		}

		if err != nil {
			// Skip error type NotFound, to request transaction result from EN
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}
		}

		txResult, err = b.getTransactionResultFromExecutionNode(ctx, block, txID, requiredEventEncodingVersion)
		if err != nil {
			// if either the execution node reported no results or the execution node could not be chosen
			if status.Code(err) == codes.NotFound {
				// No result yet, indicate that it has not been executed
				return nil, nil
			}
			// Other Error trying to retrieve the result, return with err
			return nil, err
		}

		// considered executed as long as some result is returned, even if it's an error message
		return txResult, nil
	default:
		return nil, status.Errorf(codes.Internal, "unknown transaction result query mode: %v", b.txResultQueryMode)
	}
}

func (b *backendTransactions) getHistoricalTransaction(
	ctx context.Context,
	txID flow.Identifier,
) (*flow.TransactionBody, error) {
	for _, historicalNode := range b.previousAccessNodes {
		txResp, err := historicalNode.GetTransaction(ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
		if err == nil {
			tx, err := convert.MessageToTransaction(txResp.Transaction, b.chainID.Chain())
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

func (b *backendTransactions) getHistoricalTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (*access.TransactionResult, error) {
	for _, historicalNode := range b.previousAccessNodes {
		result, err := historicalNode.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
		if err == nil {
			// Found on a historical node. Report
			if result.GetStatus() == entities.TransactionStatus_UNKNOWN {
				// We've moved to returning Status UNKNOWN instead of an error with the NotFound status,
				// Therefore we should continue and look at the next access node for answers.
				continue
			}

			if result.GetStatus() == entities.TransactionStatus_PENDING {
				// This is on a historical node. No transactions  from it will ever be
				// executed, therefore we should consider this expired
				result.Status = entities.TransactionStatus_EXPIRED
			}

			return access.MessageToTransactionResult(result), nil
		}
		// Otherwise, if not found, just continue
		if status.Code(err) == codes.NotFound {
			continue
		}
		// TODO should we do something if the error isn't not found?
	}
	return nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", txID)
}

func (b *backendTransactions) registerTransactionForRetry(tx *flow.TransactionBody) {
	referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
	if err != nil {
		return
	}

	b.retry.RegisterTransaction(referenceBlock.Height, tx)
}

func (b *backendTransactions) getTransactionResultFromExecutionNode(
	ctx context.Context,
	block *flow.Block,
	transactionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	blockID := block.ID()
	// create an execution API request for events at blockID and transactionID
	req := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID[:],
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		// if no execution receipt were found, return a NotFound GRPC error
		if IsInsufficientExecutionReceipts(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, err
	}

	resp, err := b.getTransactionResultFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, err
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := b.deriveTransactionStatus(nil, true, block)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to convert events to message", codes.Internal)
	}

	return &access.TransactionResult{
		TransactionID: transactionID,
		Status:        txStatus,
		StatusCode:    uint(resp.GetStatusCode()),
		Events:        events,
		ErrorMessage:  resp.GetErrorMessage(),
		BlockID:       blockID,
		BlockHeight:   block.Header.Height,
	}, nil
}

// ATTENTION: might be a source of problems in future. We run this code on finalization gorotuine,
// potentially lagging finalization events if operations take long time.
// We might need to move this logic on dedicated goroutine and provide a way to skip finalization events if they are delivered
// too often for this engine. An example of similar approach - https://github.com/onflow/flow-go/blob/10b0fcbf7e2031674c00f3cdd280f27bd1b16c47/engine/common/follower/compliance_engine.go#L201..
// No errors expected during normal operations.
func (b *backendTransactions) ProcessFinalizedBlockHeight(height uint64) error {
	return b.retry.Retry(height)
}

func (b *backendTransactions) getTransactionResultFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	var errToReturn error

	defer func() {
		if errToReturn != nil {
			b.log.Info().Err(errToReturn).Msg("failed to get transaction result from execution nodes")
		}
	}()

	var resp *execproto.GetTransactionResultResponse
	errToReturn = b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			resp, err = b.tryGetTransactionResult(ctx, node, req)
			if err == nil {
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Hex("transaction_id", req.GetTransactionId()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (b *backendTransactions) tryGetTransactionResult(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResult(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (b *backendTransactions) getTransactionResultsByBlockIDFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	var errToReturn error

	defer func() {
		// log the errors
		if errToReturn != nil {
			b.log.Err(errToReturn).Msg("failed to get transaction results from execution nodes")
		}
	}()

	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionResultsResponse
	errToReturn = b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			resp, err = b.tryGetTransactionResultsByBlockID(ctx, node, req)
			if err == nil {
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (b *backendTransactions) tryGetTransactionResultsByBlockID(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultsByBlockID(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (b *backendTransactions) getTransactionResultByIndexFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	var errToReturn error
	defer func() {
		if errToReturn != nil {
			b.log.Info().Err(errToReturn).Msg("failed to get transaction result from execution nodes")
		}
	}()

	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes provided")
	}

	var resp *execproto.GetTransactionResultResponse
	errToReturn = b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			resp, err = b.tryGetTransactionResultByIndex(ctx, node, req)
			if err == nil {
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Uint32("index", req.GetIndex()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (b *backendTransactions) tryGetTransactionResultByIndex(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultByIndex(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// lookupTransactionErrorMessage returns transaction error message for specified transaction.
// If an error message for transaction can be found in the cache then it will be used to serve the request, otherwise
// an RPC call will be made to the EN to fetch that error message, fetched value will be cached in the LRU cache.
// Expected errors during normal operation:
//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
//   - status.Error - remote GRPC call to EN has failed.
func (b *backendTransactions) lookupTransactionErrorMessage(
	ctx context.Context,
	blockID flow.Identifier,
	transactionID flow.Identifier,
) (string, error) {
	var cacheKey flow.Identifier
	var value string

	if b.txErrorMessagesCache != nil {
		cacheKey = flow.MakeIDFromFingerPrint(append(blockID[:], transactionID[:]...))
		value, cached := b.txErrorMessagesCache.Get(cacheKey)
		if cached {
			return value, nil
		}
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		if IsInsufficientExecutionReceipts(err) {
			return "", status.Errorf(codes.NotFound, err.Error())
		}
		return "", rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       convert.IdentifierToMessage(blockID),
		TransactionId: convert.IdentifierToMessage(transactionID),
	}

	resp, err := b.getTransactionErrorMessageFromAnyEN(ctx, execNodes, req)
	if err != nil {
		return "", fmt.Errorf("could not fetch error message from ENs: %w", err)
	}
	value = resp.ErrorMessage

	if b.txErrorMessagesCache != nil {
		b.txErrorMessagesCache.Add(cacheKey, value)
	}

	return value, nil
}

// lookupTransactionErrorMessageByIndex returns transaction error message for specified transaction using its index.
// If an error message for transaction can be found in cache then it will be used to serve the request, otherwise
// an RPC call will be made to the EN to fetch that error message, fetched value will be cached in the LRU cache.
// Expected errors during normal operation:
//   - status.Error[codes.NotFound] - transaction result for given block ID and tx index is not available.
//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
//   - status.Error - remote GRPC call to EN has failed.
func (b *backendTransactions) lookupTransactionErrorMessageByIndex(
	ctx context.Context,
	blockID flow.Identifier,
	index uint32,
) (string, error) {
	txResult, err := b.results.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return "", rpc.ConvertStorageError(err)
	}

	var cacheKey flow.Identifier
	var value string

	if b.txErrorMessagesCache != nil {
		cacheKey = flow.MakeIDFromFingerPrint(append(blockID[:], txResult.TransactionID[:]...))
		value, cached := b.txErrorMessagesCache.Get(cacheKey)
		if cached {
			return value, nil
		}
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		if IsInsufficientExecutionReceipts(err) {
			return "", status.Errorf(codes.NotFound, err.Error())
		}
		return "", rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessageByIndexRequest{
		BlockId: convert.IdentifierToMessage(blockID),
		Index:   index,
	}

	resp, err := b.getTransactionErrorMessageByIndexFromAnyEN(ctx, execNodes, req)
	if err != nil {
		return "", fmt.Errorf("could not fetch error message from ENs: %w", err)
	}
	value = resp.ErrorMessage

	if b.txErrorMessagesCache != nil {
		b.txErrorMessagesCache.Add(cacheKey, value)
	}

	return value, nil
}

// lookupTransactionErrorMessagesByBlockID returns all error messages for failed transactions by blockID.
// An RPC call will be made to the EN to fetch missing errors messages, fetched value will be cached in the LRU cache.
// Expected errors during normal operation:
//   - status.Error[codes.NotFound] - transaction results for given block ID are not available.
//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
//   - status.Error - remote GRPC call to EN has failed.
func (b *backendTransactions) lookupTransactionErrorMessagesByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) (map[flow.Identifier]string, error) {
	txResults, err := b.results.ByBlockID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	results := make(map[flow.Identifier]string)

	if b.txErrorMessagesCache != nil {
		needToFetch := false
		for _, txResult := range txResults {
			if txResult.Failed {
				cacheKey := flow.MakeIDFromFingerPrint(append(blockID[:], txResult.TransactionID[:]...))
				if value, ok := b.txErrorMessagesCache.Get(cacheKey); ok {
					results[txResult.TransactionID] = value
				} else {
					needToFetch = true
				}
			}
		}

		// all transactions were served from cache or there were no failed transactions
		if !needToFetch {
			return results, nil
		}
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		if IsInsufficientExecutionReceipts(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	resp, err := b.getTransactionErrorMessagesFromAnyEN(ctx, execNodes, req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch error message from ENs: %w", err)
	}
	result := make(map[flow.Identifier]string, len(resp))
	for _, value := range resp {
		if b.txErrorMessagesCache != nil {
			cacheKey := flow.MakeIDFromFingerPrint(append(req.BlockId, value.TransactionId...))
			b.txErrorMessagesCache.Add(cacheKey, value.ErrorMessage)
		}
		result[convert.MessageToIdentifier(value.TransactionId)] = value.ErrorMessage
	}
	return result, nil
}

// getTransactionErrorMessageFromAnyEN performs an RPC call using available nodes passed as argument. List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (b *backendTransactions) getTransactionErrorMessageFromAnyEN(
	ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetTransactionErrorMessageRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionErrorMessageResponse
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			resp, err = b.tryGetTransactionErrorMessageFromEN(ctx, node, req)
			if err == nil {
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Hex("transaction_id", req.GetTransactionId()).
					Msg("Successfully got transaction error message from any node")
				return nil
			}
			return err
		},
		nil,
	)

	// log the errors
	if errToReturn != nil {
		b.log.Err(errToReturn).Msg("failed to get transaction error message from execution nodes")
		return nil, errToReturn
	}

	return resp, nil
}

// getTransactionErrorMessageFromAnyEN performs an RPC call using available nodes passed as argument. List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (b *backendTransactions) getTransactionErrorMessageByIndexFromAnyEN(
	ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetTransactionErrorMessageByIndexRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionErrorMessageResponse
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			resp, err = b.tryGetTransactionErrorMessageByIndexFromEN(ctx, node, req)
			if err == nil {
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Uint32("index", req.GetIndex()).
					Msg("Successfully got transaction error message by index from any node")
				return nil
			}
			return err
		},
		nil,
	)
	if errToReturn != nil {
		b.log.Err(errToReturn).Msg("failed to get transaction error message by index from execution nodes")
		return nil, errToReturn
	}

	return resp, nil
}

// getTransactionErrorMessagesFromAnyEN performs an RPC call using available nodes passed as argument. List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (b *backendTransactions) getTransactionErrorMessagesFromAnyEN(
	ctx context.Context,
	execNodes flow.IdentityList,
	req *execproto.GetTransactionErrorMessagesByBlockIDRequest,
) ([]*execproto.GetTransactionErrorMessagesResponse_Result, error) {
	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionErrorMessagesResponse
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			resp, err = b.tryGetTransactionErrorMessagesByBlockIDFromEN(ctx, node, req)
			if err == nil {
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Msg("Successfully got transaction error messages from any node")
				return nil
			}
			return err
		},
		nil,
	)

	// log the errors
	if errToReturn != nil {
		b.log.Err(errToReturn).Msg("failed to get transaction error messages from execution nodes")
		return nil, errToReturn
	}

	return resp.GetResults(), nil
}

// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
//
// tryGetTransactionErrorMessageFromEN performs a grpc call to the specified execution node and returns response.
func (b *backendTransactions) tryGetTransactionErrorMessageFromEN(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetTransactionErrorMessageRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return execRPCClient.GetTransactionErrorMessage(ctx, req)
}

// tryGetTransactionErrorMessageByIndexFromEN performs a grpc call to the specified execution node and returns response.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (b *backendTransactions) tryGetTransactionErrorMessageByIndexFromEN(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetTransactionErrorMessageByIndexRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return execRPCClient.GetTransactionErrorMessageByIndex(ctx, req)
}

// tryGetTransactionErrorMessagesByBlockIDFromEN performs a grpc call to the specified execution node and returns response.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (b *backendTransactions) tryGetTransactionErrorMessagesByBlockIDFromEN(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetTransactionErrorMessagesByBlockIDRequest,
) (*execproto.GetTransactionErrorMessagesResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return execRPCClient.GetTransactionErrorMessagesByBlockID(ctx, req)
}
