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
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendTransactions struct {
	staticCollectionRPC  accessproto.AccessAPIClient // rpc client tied to a fixed collection node
	transactions         storage.Transactions
	executionReceipts    storage.ExecutionReceipts
	collections          storage.Collections
	blocks               storage.Blocks
	state                protocol.State
	chainID              flow.ChainID
	transactionMetrics   module.TransactionMetrics
	transactionValidator *access.TransactionValidator
	retry                *Retry
	connFactory          connection.ConnectionFactory

	previousAccessNodes []accessproto.AccessAPIClient
	log                 zerolog.Logger
	nodeCommunicator    Communicator
	txResultCache       *lru.Cache[flow.Identifier, *access.TransactionResult]
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
	collNodes, err := b.chooseCollectionNodes(tx)
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
func (b *backendTransactions) chooseCollectionNodes(tx *flow.TransactionBody) (flow.IdentityList, error) {

	// retrieve the set of collector clusters
	clusters, err := b.state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	targetNodes, ok := clusters.ByTxID(tx.ID())
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %x", tx.ID())
	}

	return targetNodes, nil
}

// sendTransactionToCollection sends the transaction to the given collection node via grpc
func (b *backendTransactions) sendTransactionToCollector(ctx context.Context,
	tx *flow.TransactionBody,
	collectionNodeAddr string) error {

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
	ctx context.Context,
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

	var transactionWasExecuted bool
	var events []flow.Event
	var txError string
	var statusCode uint32
	var blockHeight uint64

	// access node may not have the block if it hasn't yet been finalized, hence block can be nil at this point
	if block != nil {
		foundBlockID := block.ID()
		transactionWasExecuted, events, statusCode, txError, err = b.lookupTransactionResult(ctx, txID, foundBlockID, requiredEventEncodingVersion)
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

		blockID = foundBlockID
		blockHeight = block.Header.Height
	}

	// derive status of the transaction
	txStatus, err := b.deriveTransactionStatus(tx, transactionWasExecuted, block)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	b.transactionMetrics.TransactionResultFetched(time.Since(start), len(tx.Script))

	return &access.TransactionResult{
		Status:        txStatus,
		StatusCode:    uint(statusCode),
		Events:        events,
		ErrorMessage:  txError,
		BlockID:       blockID,
		TransactionID: txID,
		CollectionID:  collectionID,
		BlockHeight:   blockHeight,
	}, nil
}

// lookupCollectionIDInBlock returns the collection ID based on the transaction ID. The lookup is performed in block
// collections.
func (b *backendTransactions) lookupCollectionIDInBlock(
	block *flow.Block,
	txID flow.Identifier,
) (flow.Identifier, error) {
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := b.collections.LightByID(guarantee.ID())
		if err != nil {
			return flow.ZeroID, err
		}

		for _, collectionTxID := range collection.Transactions {
			if collectionTxID == txID {
				return collection.ID(), nil
			}
		}
	}
	return flow.ZeroID, status.Error(codes.NotFound, "transaction not found in block")
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

// deriveTransactionStatus derives the transaction status based on current protocol state
func (b *backendTransactions) deriveTransactionStatus(
	tx *flow.TransactionBody,
	executed bool,
	block *flow.Block,
) (flow.TransactionStatus, error) {

	if block == nil {
		// Not in a block, let's see if it's expired
		referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}
		refHeight := referenceBlock.Height
		// get the latest finalized block from the state
		finalized, err := b.state.Final().Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}
		finalizedHeight := finalized.Height

		// if we haven't seen the expiry block for this transaction, it's not expired
		if !b.isExpired(refHeight, finalizedHeight) {
			return flow.TransactionStatusPending, nil
		}

		// At this point, we have seen the expiry block for the transaction.
		// This means that, if no collections  prior to the expiry block contain
		// the transaction, it can never be included and is expired.
		//
		// To ensure this, we need to have received all collections  up to the
		// expiry block to ensure the transaction did not appear in any.

		// the last full height is the height where we have received all
		// collections  for all blocks with a lower height
		fullHeight, err := b.blocks.GetLastFullBlockHeight()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}

		// if we have received collections  for all blocks up to the expiry block, the transaction is expired
		if b.isExpired(refHeight, fullHeight) {
			return flow.TransactionStatusExpired, nil
		}

		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return flow.TransactionStatusPending, nil
	}

	if !executed {
		// If we've gotten here, but the block has not yet been executed, report it as only been finalized
		return flow.TransactionStatusFinalized, nil
	}

	// From this point on, we know for sure this transaction has at least been executed

	// get the latest sealed block from the State
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}

	if block.Header.Height > sealed.Height {
		// The block is not yet sealed, so we'll report it as only executed
		return flow.TransactionStatusExecuted, nil
	}

	// otherwise, this block has been executed, and sealed, so report as sealed
	return flow.TransactionStatusSealed, nil
}

// isExpired checks whether a transaction is expired given the height of the
// transaction's reference block and the height to compare against.
func (b *backendTransactions) isExpired(refHeight, compareToHeight uint64) bool {
	if compareToHeight <= refHeight {
		return false
	}
	return compareToHeight-refHeight > flow.DefaultTransactionExpiry
}

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
	blockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (bool, []flow.Event, uint32, string, error) {
	events, txStatus, message, err := b.getTransactionResultFromExecutionNode(ctx, blockID, txID[:], requiredEventEncodingVersion)
	if err != nil {
		// if either the execution node reported no results or the execution node could not be chosen
		if status.Code(err) == codes.NotFound {
			// No result yet, indicate that it has not been executed
			return false, nil, 0, "", nil
		}
		// Other Error trying to retrieve the result, return with err
		return false, nil, 0, "", err
	}

	// considered executed as long as some result is returned, even if it's an error message
	return true, events, txStatus, message, nil
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
	blockID flow.Identifier,
	transactionID []byte,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.Event, uint32, string, error) {

	// create an execution API request for events at blockID and transactionID
	req := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID,
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		// if no execution receipt were found, return a NotFound GRPC error
		if IsInsufficientExecutionReceipts(err) {
			return nil, 0, "", status.Errorf(codes.NotFound, err.Error())
		}
		return nil, 0, "", err
	}

	resp, err := b.getTransactionResultFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, 0, "", err
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, 0, "", rpc.ConvertError(err, "failed to convert events to message", codes.Internal)
	}

	return events, resp.GetStatusCode(), resp.GetErrorMessage(), nil
}

func (b *backendTransactions) NotifyFinalizedBlockHeight(height uint64) {
	b.retry.Retry(height)
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
