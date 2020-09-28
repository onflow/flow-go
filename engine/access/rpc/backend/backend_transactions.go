package backend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/go-multierror"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
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
	executionRPC         execproto.ExecutionAPIClient
	transactions         storage.Transactions
	collections          storage.Collections
	blocks               storage.Blocks
	state                protocol.State
	chainID              flow.ChainID
	transactionMetrics   module.TransactionMetrics
	transactionValidator *access.TransactionValidator
	retry                *Retry
	collectionGRPCPort   uint
	connFactory          ConnectionFactory
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

	var sendErrors error

	// try sending the transaction to one of the chosen collection nodes
	for _, addr := range collAddrs {
		err = b.sendTransactionToCollector(ctx, tx, addr)
		if err != nil {
			sendErrors = multierror.Append(sendErrors, err)
		} else {
			return nil
		}
	}
	return sendErrors
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

	// convert the node addresses of the collection nodes to the GRPC address
	// (identity list does not directly provide collection nodes gRPC address)
	var targetAddrs = make([]string, len(targetNodes))
	for i, id := range targetNodes {
		// split hostname and port
		hostnameOrIP, _, err := net.SplitHostPort(id.Address)
		if err != nil {
			return nil, err
		}
		// use the hostname from identity list and port number as the one passed in as argument
		grpcAddress := fmt.Sprintf("%s:%d", hostnameOrIP, b.collectionGRPCPort)
		targetAddrs[i] = grpcAddress
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

func (b *backendTransactions) GetTransaction(_ context.Context, txID flow.Identifier) (*flow.TransactionBody, error) {
	// look up transaction from storage
	tx, err := b.transactions.ByID(txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	return tx, nil
}

func (b *backendTransactions) GetTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (*access.TransactionResult, error) {
	// look up transaction from storage
	tx, err := b.transactions.ByID(txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// get events for the transaction
	executed, events, statusCode, txError, err := b.lookupTransactionResult(ctx, txID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// derive status of the transaction
	status, err := b.DeriveTransactionStatus(tx, executed)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// TODO: Set correct values for StatusCode and ErrorMessage

	return &access.TransactionResult{
		Status:       status,
		StatusCode:   uint(statusCode),
		Events:       events,
		ErrorMessage: txError,
	}, nil
}

// DeriveTransactionStatus derives the transaction status based on current protocol state
func (b *backendTransactions) DeriveTransactionStatus(
	tx *flow.TransactionBody,
	executed bool,
) (flow.TransactionStatus, error) {

	block, err := b.lookupBlock(tx.ID())
	if errors.Is(err, storage.ErrNotFound) {
		// Not in a block, let's see if it's expired
		referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}
		// get the latest finalized block from the state
		finalized, err := b.state.Final().Head()
		if err != nil {
			return flow.TransactionStatusUnknown, err
		}

		// Have to check if finalized height is greater than reference block height rather than rely on the subtraction, since
		// heights are unsigned ints
		if finalized.Height > referenceBlock.Height && finalized.Height-referenceBlock.Height > flow.DefaultTransactionExpiry {
			return flow.TransactionStatusExpired, err
		}

		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return flow.TransactionStatusPending, nil
	}
	if err != nil {
		return flow.TransactionStatusUnknown, err
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
) (bool, []flow.Event, uint32, string, error) {

	// find the block ID for the transaction
	block, err := b.lookupBlock(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// access node may not have the block if it hasn't yet been finalized
			return false, nil, 0, "", nil
		}
		return false, nil, 0, "", convertStorageError(err)
	}

	blockID := block.ID()

	events, txStatus, message, err := b.getTransactionResultFromExecutionNode(ctx, blockID[:], txID[:])
	if err != nil {
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

func (b *backendTransactions) registerTransactionForRetry(tx *flow.TransactionBody) {
	referenceBlock, err := b.state.AtBlockID(tx.ReferenceBlockID).Head()
	if err != nil {
		return
	}

	b.retry.RegisterTransaction(referenceBlock.Height, tx)
}

func (b *backendTransactions) getTransactionResultFromExecutionNode(
	ctx context.Context,
	blockID []byte,
	transactionID []byte,
) ([]flow.Event, uint32, string, error) {

	// create an execution API request for events at blockID and transactionID
	req := execproto.GetTransactionResultRequest{
		BlockId:       blockID,
		TransactionId: transactionID,
	}

	// call the execution node gRPC
	resp, err := b.executionRPC.GetTransactionResult(ctx, &req)
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
