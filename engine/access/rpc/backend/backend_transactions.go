package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const collectionNodesToTry uint = 3

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
	connFactory          ConnectionFactory

	previousAccessNodes []accessproto.AccessAPIClient
	log                 zerolog.Logger
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
		return status.Error(codes.Internal, fmt.Sprintf("failed to send transaction to a collection node: %v", err))
	}

	b.transactionMetrics.TransactionReceived(tx.ID(), now)

	// store the transaction locally
	err = b.transactions.Store(tx)
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
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

	// otherwise choose a random set of collections nodes to try
	collAddrs, err := b.chooseCollectionNodes(tx, collectionNodesToTry)
	if err != nil {
		return fmt.Errorf("failed to determine collection node for tx %x: %w", tx, err)
	}

	var sendErrors *multierror.Error
	logAnyError := func() {
		err = sendErrors.ErrorOrNil()
		if err != nil {
			b.log.Info().Err(err).Msg("failed to send transactions to collector nodes")
		}
	}
	defer logAnyError()

	// try sending the transaction to one of the chosen collection nodes
	for _, addr := range collAddrs {
		err = b.sendTransactionToCollector(ctx, tx, addr)
		if err == nil {
			return nil
		}
		sendErrors = multierror.Append(sendErrors, err)
	}

	return sendErrors.ErrorOrNil()
}

// chooseCollectionNodes finds a random subset of size sampleSize of collection node addresses from the
// collection node cluster responsible for the given tx
func (b *backendTransactions) chooseCollectionNodes(tx *flow.TransactionBody, sampleSize uint) ([]string, error) {

	// retrieve the set of collector clusters
	clusters, err := b.state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(tx.ID())
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %x", tx.ID())
	}

	// select a random subset of collection nodes from the cluster to be tried in order
	targetNodes := txCluster.Sample(sampleSize)

	// collect the addresses of all the chosen collection nodes
	var targetAddrs = make([]string, len(targetNodes))
	for i, id := range targetNodes {
		targetAddrs[i] = id.Address
	}

	return targetAddrs, nil
}

// sendTransactionToCollection sends the transaction to the given collection node via grpc
func (b *backendTransactions) sendTransactionToCollector(ctx context.Context,
	tx *flow.TransactionBody,
	collectionNodeAddr string) error {

	// TODO: Use a connection pool to cache connections
	collectionRPC, conn, err := b.connFactory.GetAccessAPIClient(collectionNodeAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to collection node at %s: %w", collectionNodeAddr, err)
	}
	defer conn.Close()

	err = b.grpcTxSend(ctx, collectionRPC, tx)
	if err != nil {
		return fmt.Errorf("failed to send transaction to collection node at %s: %v", collectionNodeAddr, err)
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
	txErr := convertStorageError(err)
	if txErr != nil {
		if status.Code(txErr) == codes.NotFound {
			return b.getHistoricalTransaction(ctx, txID)
		}
		// Other Error trying to retrieve the transaction, return with err
		return nil, txErr
	}

	return tx, nil
}

func (b *backendTransactions) GetTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (*access.TransactionResult, error) {
	// look up transaction from storage
	tx, err := b.transactions.ByID(txID)
	txErr := convertStorageError(err)
	if txErr != nil {
		if status.Code(txErr) == codes.NotFound {
			// Tx not found. If we have historical Sporks setup, lets look through those as well
			historicalTxResult, err := b.getHistoricalTransactionResult(ctx, txID)
			if err != nil {
				// if tx not found in old access nodes either, then assume that the tx was submitted to a different AN
				// and return status as unknown
				status := flow.TransactionStatusUnknown
				return &access.TransactionResult{
					Status:     status,
					StatusCode: uint(status),
				}, nil
			}
			return historicalTxResult, nil
		}
		return nil, txErr
	}

	// find the block for the transaction
	block, err := b.lookupBlock(txID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, convertStorageError(err)
	}

	var blockID flow.Identifier
	var transactionWasExecuted bool
	var events []flow.Event
	var txError string
	var statusCode uint32
	// access node may not have the block if it hasn't yet been finalized, hence block can be nil at this point
	if block != nil {
		blockID = block.ID()
		transactionWasExecuted, events, statusCode, txError, err = b.lookupTransactionResult(ctx, txID, blockID)
		if err != nil {
			return nil, convertStorageError(err)
		}
	}

	// derive status of the transaction
	status, err := b.deriveTransactionStatus(tx, transactionWasExecuted, block)
	if err != nil {
		return nil, convertStorageError(err)
	}

	return &access.TransactionResult{
		Status:       status,
		StatusCode:   uint(statusCode),
		Events:       events,
		ErrorMessage: txError,
		BlockID:      blockID,
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
		// This means that, if no collections prior to the expiry block contain
		// the transaction, it can never be included and is expired.
		//
		// To ensure this, we need to have received all collections up to the
		// expiry block to ensure the transaction did not appear in any.

		// the last full height is the height where we have received all
		// collections for all blocks with a lower height
		fullHeight, err := b.blocks.GetLastFullBlockHeight()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}

		// if we have received collections for all blocks up to the expiry block, the transaction is expired
		if b.isExpired(refHeight, fullHeight) {
			return flow.TransactionStatusExpired, err
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

	// get the latest sealed block from the state
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
) (bool, []flow.Event, uint32, string, error) {

	events, txStatus, message, err := b.getTransactionResultFromExecutionNode(ctx, blockID, txID[:])
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
			// Found on a historical node. Report
			return &tx, err
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
			if result.GetStatus() == entities.TransactionStatus_PENDING {
				// This is on a historical node. No transactions from it will ever be
				// executed, therefore we should consider this expired
				result.Status = entities.TransactionStatus_EXPIRED
			} else if result.GetStatus() == entities.TransactionStatus_UNKNOWN {
				// We've moved to returning Status UNKNOWN instead of an error with the NotFound status,
				// Therefore we should continue and look at the next access node for answers.
				continue
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
) ([]flow.Event, uint32, string, error) {

	// create an execution API request for events at blockID and transactionID
	req := execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID,
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		// if no execution receipt were found, return a NotFound GRPC error
		if errors.As(err, &InsufficientExecutionReceipts{}) {
			return nil, 0, "", status.Errorf(codes.NotFound, err.Error())
		}
		return nil, 0, "", status.Errorf(codes.Internal, "failed to retrieve result from any execution node: %v", err)
	}

	resp, err := b.getTransactionResultFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, 0, "", err
		}
		return nil, 0, "", status.Errorf(codes.Internal, "failed to retrieve result from execution node: %v", err)
	}

	events := convert.MessagesToEvents(resp.GetEvents())

	return events, resp.GetStatusCode(), resp.GetErrorMessage(), nil
}

func (b *backendTransactions) NotifyFinalizedBlockHeight(height uint64) {
	b.retry.Retry(height)
}

func (b *backendTransactions) getTransactionResultFromAnyExeNode(ctx context.Context, execNodes flow.IdentityList, req execproto.GetTransactionResultRequest) (*execproto.GetTransactionResultResponse, error) {
	var errors *multierror.Error
	// try to execute the script on one of the execution nodes
	for _, execNode := range execNodes {
		resp, err := b.tryGetTransactionResult(ctx, execNode, req)
		if err == nil {
			b.log.Debug().
				Str("execution_node", execNode.String()).
				Hex("block_id", req.GetBlockId()).
				Hex("transaction_id", req.GetTransactionId()).
				Msg("Successfully got transaction results from any node")
			return resp, nil
		}
		if status.Code(err) == codes.NotFound {
			return nil, err
		}
		errors = multierror.Append(errors, err)
	}
	return nil, errors.ErrorOrNil()
}

func (b *backendTransactions) tryGetTransactionResult(ctx context.Context, execNode *flow.Identity, req execproto.GetTransactionResultRequest) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	resp, err := execRPCClient.GetTransactionResult(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
