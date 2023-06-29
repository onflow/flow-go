package apiproxy

import (
	"context"
	"time"

	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/common/grpc/forwarder"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
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
	identities flow.IdentityList,
	timeout time.Duration,
	maxMsgSize uint,
	log zerolog.Logger,
	metrics metrics.ObserverMetrics,
	chain flow.Chain,
) (*RestProxyHandler, error) {

	forwarder, err := forwarder.NewForwarder(
		identities,
		timeout,
		maxMsgSize)
	if err != nil {
		return nil, err
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

// GetLatestBlockHeader returns the latest block header and block status, if isSealed = true - returns the latest seal header.
func (r *RestProxyHandler) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, flow.BlockStatusUnknown, err
	}

	getLatestBlockHeaderRequest := &accessproto.GetLatestBlockHeaderRequest{
		IsSealed: isSealed,
	}
	latestBlockHeaderResponse, err := upstream.GetLatestBlockHeader(ctx, getLatestBlockHeaderRequest)
	if err != nil {
		return nil, flow.BlockStatusUnknown, err
	}
	blockHeader, err := convert.MessageToBlockHeader(latestBlockHeaderResponse.Block)
	if err != nil {
		return nil, flow.BlockStatusUnknown, err
	}
	blockStatus, err := convert.MessagesToBlockStatus(latestBlockHeaderResponse.BlockStatus)
	if err != nil {
		return nil, flow.BlockStatusUnknown, err
	}

	r.log("upstream", "GetLatestBlockHeader", err)
	return blockHeader, blockStatus, nil
}

// GetCollectionByID returns a collection by ID.
func (r *RestProxyHandler) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	getCollectionByIDRequest := &accessproto.GetCollectionByIDRequest{
		Id: id[:],
	}

	collectionResponse, err := upstream.GetCollectionByID(ctx, getCollectionByIDRequest)
	if err != nil {
		return nil, err
	}

	transactions := make([]flow.Identifier, len(collectionResponse.Collection.TransactionIds))
	for _, txId := range collectionResponse.Collection.TransactionIds {
		transactions = append(transactions, convert.MessageToIdentifier(txId))
	}

	r.log("upstream", "GetCollectionByID", err)
	return &flow.LightCollection{
		Transactions: transactions,
	}, nil
}

// SendTransaction sends already created transaction.
func (r *RestProxyHandler) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return err
	}

	transaction := convert.TransactionToMessage(*tx)
	sendTransactionRequest := &accessproto.SendTransactionRequest{
		Transaction: transaction,
	}

	_, err = upstream.SendTransaction(ctx, sendTransactionRequest)
	if err != nil {
		return err
	}

	r.log("upstream", "SendTransaction", err)
	return nil

}

// GetTransaction returns transaction by ID.
func (r *RestProxyHandler) GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	getTransactionRequest := &accessproto.GetTransactionRequest{
		Id: id[:],
	}
	transactionResponse, err := upstream.GetTransaction(ctx, getTransactionRequest)
	if err != nil {
		return nil, err
	}

	transactionBody, err := convert.MessageToTransaction(transactionResponse.Transaction, r.Chain)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "GetTransaction", err)
	return &transactionBody, nil
}

// GetTransactionResult returns transaction result by the transaction ID.
func (r *RestProxyHandler) GetTransactionResult(ctx context.Context, id flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier) (*access.TransactionResult, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	getTransactionResultRequest := &accessproto.GetTransactionRequest{
		Id:           id[:],
		BlockId:      blockID[:],
		CollectionId: collectionID[:],
	}

	transactionResultResponse, err := upstream.GetTransactionResult(ctx, getTransactionResultRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "GetTransactionResult", err)
	return access.MessageToTransactionResult(transactionResultResponse), nil
}

// GetAccountAtBlockHeight returns account by account address and block height.
func (r *RestProxyHandler) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	getAccountAtBlockHeightRequest := &accessproto.GetAccountAtBlockHeightRequest{
		Address:     address.Bytes(),
		BlockHeight: height,
	}

	accountResponse, err := upstream.GetAccountAtBlockHeight(ctx, getAccountAtBlockHeightRequest)
	if err != nil {
		return nil, models.NewNotFoundError("not found account at block height", err)
	}

	r.log("upstream", "GetAccountAtBlockHeight", err)
	return convert.MessageToAccount(accountResponse.Account)
}

// ExecuteScriptAtLatestBlock executes script at latest block.
func (r *RestProxyHandler) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	executeScriptAtLatestBlockRequest := &accessproto.ExecuteScriptAtLatestBlockRequest{
		Script:    script,
		Arguments: arguments,
	}
	executeScriptAtLatestBlockResponse, err := upstream.ExecuteScriptAtLatestBlock(ctx, executeScriptAtLatestBlockRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "ExecuteScriptAtLatestBlock", err)
	return executeScriptAtLatestBlockResponse.Value, nil
}

// ExecuteScriptAtBlockHeight executes script at the given block height .
func (r *RestProxyHandler) ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	executeScriptAtBlockHeightRequest := &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: blockHeight,
		Script:      script,
		Arguments:   arguments,
	}
	executeScriptAtBlockHeightResponse, err := upstream.ExecuteScriptAtBlockHeight(ctx, executeScriptAtBlockHeightRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "ExecuteScriptAtBlockHeight", err)
	return executeScriptAtBlockHeightResponse.Value, nil
}

// ExecuteScriptAtBlockID executes script at the given block id .
func (r *RestProxyHandler) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	executeScriptAtBlockIDRequest := &accessproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}
	executeScriptAtBlockIDResponse, err := upstream.ExecuteScriptAtBlockID(ctx, executeScriptAtBlockIDRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "ExecuteScriptAtBlockID", err)
	return executeScriptAtBlockIDResponse.Value, nil
}

// GetEventsForHeightRange returns events by their name in the specified blocks heights.
func (r *RestProxyHandler) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64) ([]flow.BlockEvents, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	getEventsForHeightRangeRequest := &accessproto.GetEventsForHeightRangeRequest{
		Type:        eventType,
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}
	eventsResponse, err := upstream.GetEventsForHeightRange(ctx, getEventsForHeightRangeRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "GetEventsForHeightRange", err)
	return convert.MessagesToBlockEvents(eventsResponse.Results), nil
}

// GetEventsForBlockIDs returns events by their name in the specified block IDs.
func (r *RestProxyHandler) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier) ([]flow.BlockEvents, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	var blockIds [][]byte
	for _, id := range blockIDs {
		blockIds = append(blockIds, id[:])
	}

	getEventsForBlockIDsRequest := &accessproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: blockIds,
	}
	eventsResponse, err := upstream.GetEventsForBlockIDs(ctx, getEventsForBlockIDsRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "GetEventsForBlockIDs", err)
	return convert.MessagesToBlockEvents(eventsResponse.Results), nil
}

// GetExecutionResultForBlockID gets execution result by provided block ID.
func (r *RestProxyHandler) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	getExecutionResultForBlockID := &accessproto.GetExecutionResultForBlockIDRequest{
		BlockId: blockID[:],
	}
	executionResultForBlockIDResponse, err := upstream.GetExecutionResultForBlockID(ctx, getExecutionResultForBlockID)
	if err != nil {
		return nil, err
	}

	r.log("upsteram", "GetExecutionResultForBlockID", err)
	return convert.MessageToExecutionResult(executionResultForBlockIDResponse.ExecutionResult)
}

// GetExecutionResultByID gets execution result by its ID.
func (r *RestProxyHandler) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	upstream, err := r.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	executionResultByIDRequest := &accessproto.GetExecutionResultByIDRequest{
		Id: id[:],
	}

	executionResultByIDResponse, err := upstream.GetExecutionResultByID(ctx, executionResultByIDRequest)
	if err != nil {
		return nil, err
	}

	r.log("upstream", "GetExecutionResultByID", err)
	return convert.MessageToExecutionResult(executionResultByIDResponse.ExecutionResult)
}
