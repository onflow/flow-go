package apiproxy

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/status"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/grpc/forwarder"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

// RestProxyHandler is a structure that represents the proxy algorithm for observer node.
// It includes the local backend and forwards the methods which can't be handled locally to an upstream using gRPC API.
type RestProxyHandler struct {
	access.API
	*forwarder.Forwarder
	Logger  zerolog.Logger
	Metrics metrics.ObserverMetrics
	Chain   flow.Chain
}

// NewRestProxyHandler returns a new rest proxy handler for observer node.
func NewRestProxyHandler(
	api access.API,
	identities flow.IdentitySkeletonList,
	connectionFactory connection.ConnectionFactory,
	log zerolog.Logger,
	metrics metrics.ObserverMetrics,
	chain flow.Chain,
) (*RestProxyHandler, error) {
	forwarder, err := forwarder.NewForwarder(
		identities,
		connectionFactory,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create REST forwarder: %w", err)
	}

	restProxyHandler := &RestProxyHandler{
		Logger:  log,
		Metrics: metrics,
		Chain:   chain,
	}

	restProxyHandler.API = api
	restProxyHandler.Forwarder = forwarder

	return restProxyHandler, nil
}

func (r *RestProxyHandler) log(handler, rpc string, err error) {
	code := status.Code(err)
	r.Metrics.RecordRPC(handler, rpc, code)

	logger := r.Logger.With().
		Str("handler", handler).
		Str("rest_method", rpc).
		Str("rest_code", code.String()).
		Logger()

	if err != nil {
		logger.Error().Err(err).Msg("request failed")
		return
	}

	logger.Info().Msg("request succeeded")
}

// GetCollectionByID returns a collection by ID.
func (r *RestProxyHandler) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getCollectionByIDRequest := &accessproto.GetCollectionByIDRequest{
		Id: id[:],
	}

	collectionResponse, err := upstream.GetCollectionByID(ctx, getCollectionByIDRequest)
	r.log("upstream", "GetCollectionByID", err)

	if err != nil {
		return nil, err
	}

	transactions, err := convert.MessageToLightCollection(collectionResponse.Collection)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

// SendTransaction sends already created transaction.
func (r *RestProxyHandler) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return err
	}
	defer closer.Close()

	transaction := convert.TransactionToMessage(*tx)
	sendTransactionRequest := &accessproto.SendTransactionRequest{
		Transaction: transaction,
	}

	_, err = upstream.SendTransaction(ctx, sendTransactionRequest)
	r.log("upstream", "SendTransaction", err)

	return err
}

// GetTransaction returns transaction by ID.
func (r *RestProxyHandler) GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getTransactionRequest := &accessproto.GetTransactionRequest{
		Id: id[:],
	}
	transactionResponse, err := upstream.GetTransaction(ctx, getTransactionRequest)
	r.log("upstream", "GetTransaction", err)

	if err != nil {
		return nil, err
	}

	transactionBody, err := convert.MessageToTransaction(transactionResponse.Transaction, r.Chain)
	if err != nil {
		return nil, err
	}

	return &transactionBody, nil
}

// GetTransactionResult returns transaction result by the transaction ID.
func (r *RestProxyHandler) GetTransactionResult(
	ctx context.Context,
	id flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {

		return nil, err
	}
	defer closer.Close()

	getTransactionResultRequest := &accessproto.GetTransactionRequest{
		Id:                   id[:],
		BlockId:              blockID[:],
		CollectionId:         collectionID[:],
		EventEncodingVersion: requiredEventEncodingVersion,
	}

	transactionResultResponse, err := upstream.GetTransactionResult(ctx, getTransactionResultRequest)
	r.log("upstream", "GetTransactionResult", err)

	if err != nil {
		return nil, err
	}

	return convert.MessageToTransactionResult(transactionResultResponse), nil
}

// GetAccountAtBlockHeight returns account by account address and block height.
func (r *RestProxyHandler) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getAccountAtBlockHeightRequest := &accessproto.GetAccountAtBlockHeightRequest{
		Address:     address.Bytes(),
		BlockHeight: height,
	}

	accountResponse, err := upstream.GetAccountAtBlockHeight(ctx, getAccountAtBlockHeightRequest)
	r.log("upstream", "GetAccountAtBlockHeight", err)

	if err != nil {
		return nil, err
	}

	return convert.MessageToAccount(accountResponse.Account)
}

// GetAccountBalanceAtBlockHeight returns account balance by account address and block height.
func (r *RestProxyHandler) GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	getAccountBalanceAtBlockHeightRequest := &accessproto.GetAccountBalanceAtBlockHeightRequest{
		Address:     address.Bytes(),
		BlockHeight: height,
	}

	accountBalanceResponse, err := upstream.GetAccountBalanceAtBlockHeight(ctx, getAccountBalanceAtBlockHeightRequest)
	r.log("upstream", "GetAccountBalanceAtBlockHeight", err)

	if err != nil {
		return 0, err
	}

	return accountBalanceResponse.GetBalance(), nil

}

// GetAccountKeys returns account keys by account address and block height.
func (r *RestProxyHandler) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getAccountKeysAtBlockHeightRequest := &accessproto.GetAccountKeysAtBlockHeightRequest{
		Address:     address.Bytes(),
		BlockHeight: height,
	}

	accountKeyResponse, err := upstream.GetAccountKeysAtBlockHeight(ctx, getAccountKeysAtBlockHeightRequest)
	r.log("upstream", "GetAccountKeysAtBlockHeight", err)

	if err != nil {
		return nil, err
	}

	accountKeys := make([]flow.AccountPublicKey, len(accountKeyResponse.GetAccountKeys()))
	for i, key := range accountKeyResponse.GetAccountKeys() {
		accountKey, err := convert.MessageToAccountKey(key)
		if err != nil {
			return nil, err
		}

		accountKeys[i] = *accountKey
	}

	return accountKeys, nil
}

// GetAccountKeyByIndex returns account key by account address, key index and block height.
func (r *RestProxyHandler) GetAccountKeyByIndex(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getAccountKeyAtBlockHeightRequest := &accessproto.GetAccountKeyAtBlockHeightRequest{
		Address:     address.Bytes(),
		Index:       keyIndex,
		BlockHeight: height,
	}

	accountKeyResponse, err := upstream.GetAccountKeyAtBlockHeight(ctx, getAccountKeyAtBlockHeightRequest)
	r.log("upstream", "GetAccountKeyAtBlockHeight", err)

	if err != nil {
		return nil, err
	}

	return convert.MessageToAccountKey(accountKeyResponse.AccountKey)
}

// ExecuteScriptAtLatestBlock executes script at latest block.
func (r *RestProxyHandler) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	executeScriptAtLatestBlockRequest := &accessproto.ExecuteScriptAtLatestBlockRequest{
		Script:    script,
		Arguments: arguments,
	}
	executeScriptAtLatestBlockResponse, err := upstream.ExecuteScriptAtLatestBlock(ctx, executeScriptAtLatestBlockRequest)
	r.log("upstream", "ExecuteScriptAtLatestBlock", err)

	if err != nil {
		return nil, err
	}

	return executeScriptAtLatestBlockResponse.Value, nil
}

// ExecuteScriptAtBlockHeight executes script at the given block height .
func (r *RestProxyHandler) ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	executeScriptAtBlockHeightRequest := &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: blockHeight,
		Script:      script,
		Arguments:   arguments,
	}
	executeScriptAtBlockHeightResponse, err := upstream.ExecuteScriptAtBlockHeight(ctx, executeScriptAtBlockHeightRequest)
	r.log("upstream", "ExecuteScriptAtBlockHeight", err)

	if err != nil {
		return nil, err
	}

	return executeScriptAtBlockHeightResponse.Value, nil
}

// ExecuteScriptAtBlockID executes script at the given block id .
func (r *RestProxyHandler) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	executeScriptAtBlockIDRequest := &accessproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}
	executeScriptAtBlockIDResponse, err := upstream.ExecuteScriptAtBlockID(ctx, executeScriptAtBlockIDRequest)
	r.log("upstream", "ExecuteScriptAtBlockID", err)

	if err != nil {
		return nil, err
	}

	return executeScriptAtBlockIDResponse.Value, nil
}

// GetEventsForHeightRange returns events by their name in the specified blocks heights.
func (r *RestProxyHandler) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getEventsForHeightRangeRequest := &accessproto.GetEventsForHeightRangeRequest{
		Type:                 eventType,
		StartHeight:          startHeight,
		EndHeight:            endHeight,
		EventEncodingVersion: requiredEventEncodingVersion,
	}
	eventsResponse, err := upstream.GetEventsForHeightRange(ctx, getEventsForHeightRangeRequest)
	r.log("upstream", "GetEventsForHeightRange", err)

	if err != nil {
		return nil, err
	}

	return convert.MessagesToBlockEvents(eventsResponse.Results), nil
}

// GetEventsForBlockIDs returns events by their name in the specified block IDs.
func (r *RestProxyHandler) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	blockIds := convert.IdentifiersToMessages(blockIDs)

	getEventsForBlockIDsRequest := &accessproto.GetEventsForBlockIDsRequest{
		Type:                 eventType,
		BlockIds:             blockIds,
		EventEncodingVersion: requiredEventEncodingVersion,
	}
	eventsResponse, err := upstream.GetEventsForBlockIDs(ctx, getEventsForBlockIDsRequest)
	r.log("upstream", "GetEventsForBlockIDs", err)

	if err != nil {
		return nil, err
	}

	return convert.MessagesToBlockEvents(eventsResponse.Results), nil
}

// GetExecutionResultForBlockID gets execution result by provided block ID.
func (r *RestProxyHandler) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	getExecutionResultForBlockID := &accessproto.GetExecutionResultForBlockIDRequest{
		BlockId: blockID[:],
	}
	executionResultForBlockIDResponse, err := upstream.GetExecutionResultForBlockID(ctx, getExecutionResultForBlockID)
	r.log("upstream", "GetExecutionResultForBlockID", err)

	if err != nil {
		return nil, err
	}

	return convert.MessageToExecutionResult(executionResultForBlockIDResponse.ExecutionResult)
}

// GetExecutionResultByID gets execution result by its ID.
func (r *RestProxyHandler) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	executionResultByIDRequest := &accessproto.GetExecutionResultByIDRequest{
		Id: id[:],
	}

	executionResultByIDResponse, err := upstream.GetExecutionResultByID(ctx, executionResultByIDRequest)
	r.log("upstream", "GetExecutionResultByID", err)

	if err != nil {
		return nil, err
	}

	return convert.MessageToExecutionResult(executionResultByIDResponse.ExecutionResult)
}
