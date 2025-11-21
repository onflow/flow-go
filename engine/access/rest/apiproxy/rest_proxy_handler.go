package apiproxy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/grpc/forwarder"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/metrics"
)

// RestProxyHandler implements the access.API interface to provide backend functionality for the
// REST API. It uses a local implementation for all data that is available locally on observer nodes,
// and proxies the remaining methods to an upstream Access node using its grpc endpoints.
// For proxied methods, the handler converts the grpc response and errors back to the local
// representation and returns them to the caller.
type RestProxyHandler struct {
	access.API
	*forwarder.Forwarder
	logger  zerolog.Logger
	metrics metrics.ObserverMetrics
	chain   flow.Chain
}

var _ access.API = (*RestProxyHandler)(nil)

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
		logger:  log,
		metrics: metrics,
		chain:   chain,
	}

	restProxyHandler.API = api
	restProxyHandler.Forwarder = forwarder

	return restProxyHandler, nil
}

func (r *RestProxyHandler) log(handler, rpc string, err error) {
	code := status.Code(err)
	r.metrics.RecordRPC(handler, rpc, code)

	logger := r.logger.With().
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

// GetCollectionByID returns a light collection by its ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.ServiceUnavailable]: If the configured upstream access client failed to respond.
//   - [access.DataNotFoundError]: If the collection is not found.
func (r *RestProxyHandler) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, access.NewServiceUnavailable(err)
	}
	defer closer.Close()

	getCollectionByIDRequest := &accessproto.GetCollectionByIDRequest{
		Id: id[:],
	}

	collectionResponse, err := upstream.GetCollectionByID(ctx, getCollectionByIDRequest)
	r.log("upstream", "GetCollectionByID", err)

	if err != nil {
		return nil, convertError(ctx, err, "collection")
	}

	collection, err := convert.MessageToLightCollection(collectionResponse.Collection)
	if err != nil {
		// this is not fatal because the data is coming from the upstream AN, so a failure here
		// does not imply inconsistent local state.
		return nil, access.NewInternalError(fmt.Errorf("failed to convert collection response: %w", err))
	}

	return collection, nil
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

	transactionBody, err := convert.MessageToTransaction(transactionResponse.Transaction, r.chain)
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

	return convert.MessageToTransactionResult(transactionResultResponse)
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
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.InvalidRequestError]: If the request had invalid arguments.
//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
//   - [access.DataNotFoundError]: If data required to process the request is not available.
//   - [access.OutOfRangeError]: If the requested data is outside the available range.
//   - [access.PreconditionFailedError]: If data for block is not available.
//   - [access.RequestCanceledError]: If the script execution was canceled.
//   - [access.RequestTimedOutError]: If the script execution timed out.
//   - [access.ServiceUnavailable]: If configured to use an external node for script execution and
//     no upstream server is available.
//   - [access.InternalError]: For internal failures or index conversion errors.
func (r *RestProxyHandler) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte, criteria optimistic_sync.Criteria) ([]byte, *accessmodel.ExecutorMetadata, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, nil, access.NewServiceUnavailable(err)
	}
	defer closer.Close()

	executeScriptAtLatestBlockRequest := &accessproto.ExecuteScriptAtLatestBlockRequest{
		Script:              script,
		Arguments:           arguments,
		ExecutionStateQuery: executionStateQuery(criteria),
	}
	executeScriptAtLatestBlockResponse, err := upstream.ExecuteScriptAtLatestBlock(ctx, executeScriptAtLatestBlockRequest)
	r.log("upstream", "ExecuteScriptAtLatestBlock", err)

	if err != nil {
		return nil, nil, convertError(ctx, err, "register")
	}

	var metadata *accessmodel.ExecutorMetadata
	if rawMetadata := executeScriptAtLatestBlockResponse.GetMetadata(); rawMetadata != nil {
		metadata = convert.MessageToExecutorMetadata(rawMetadata.GetExecutorMetadata())
	}

	return executeScriptAtLatestBlockResponse.Value, metadata, nil
}

// ExecuteScriptAtBlockHeight executes script at the given block height.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.InvalidRequestError]: If the request had invalid arguments.
//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
//   - [access.DataNotFoundError]: If data required to process the request is not available.
//   - [access.OutOfRangeError]: If the requested data is outside the available range.
//   - [access.PreconditionFailedError]: If data for block is not available.
//   - [access.RequestCanceledError]: If the script execution was canceled.
//   - [access.RequestTimedOutError]: If the script execution timed out.
//   - [access.ServiceUnavailable]: If configured to use an external node for script execution and
//     no upstream server is available.
//   - [access.InternalError]: For internal failures or index conversion errors.
func (r *RestProxyHandler) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
	criteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, nil, access.NewServiceUnavailable(err)
	}
	defer closer.Close()

	executeScriptAtBlockHeightRequest := &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight:         blockHeight,
		Script:              script,
		Arguments:           arguments,
		ExecutionStateQuery: executionStateQuery(criteria),
	}
	executeScriptAtBlockHeightResponse, err := upstream.ExecuteScriptAtBlockHeight(ctx, executeScriptAtBlockHeightRequest)
	r.log("upstream", "ExecuteScriptAtBlockHeight", err)

	if err != nil {
		return nil, nil, convertError(ctx, err, "register")
	}

	var metadata *accessmodel.ExecutorMetadata
	if rawMetadata := executeScriptAtBlockHeightResponse.GetMetadata(); rawMetadata != nil {
		metadata = convert.MessageToExecutorMetadata(rawMetadata.GetExecutorMetadata())
	}

	return executeScriptAtBlockHeightResponse.Value, metadata, nil
}

// ExecuteScriptAtBlockID executes script at the given block id.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.InvalidRequestError]: If the request had invalid arguments.
//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
//   - [access.DataNotFoundError]: If data required to process the request is not available.
//   - [access.OutOfRangeError]: If the requested data is outside the available range.
//   - [access.PreconditionFailedError]: If data for block is not available.
//   - [access.RequestCanceledError]: If the script execution was canceled.
//   - [access.RequestTimedOutError]: If the script execution timed out.
//   - [access.ServiceUnavailable]: If configured to use an external node for script execution and
//     no upstream server is available.
//   - [access.InternalError]: For internal failures or index conversion errors.
func (r *RestProxyHandler) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	criteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, nil, access.NewServiceUnavailable(err)
	}
	defer closer.Close()

	executeScriptAtBlockIDRequest := &accessproto.ExecuteScriptAtBlockIDRequest{
		BlockId:             blockID[:],
		Script:              script,
		Arguments:           arguments,
		ExecutionStateQuery: executionStateQuery(criteria),
	}
	executeScriptAtBlockIDResponse, err := upstream.ExecuteScriptAtBlockID(ctx, executeScriptAtBlockIDRequest)
	r.log("upstream", "ExecuteScriptAtBlockID", err)

	if err != nil {
		return nil, nil, convertError(ctx, err, "register")
	}

	var metadata *accessmodel.ExecutorMetadata
	if rawMetadata := executeScriptAtBlockIDResponse.GetMetadata(); rawMetadata != nil {
		metadata = convert.MessageToExecutorMetadata(rawMetadata.GetExecutorMetadata())
	}

	return executeScriptAtBlockIDResponse.Value, metadata, nil
}

// GetEventsForHeightRange returns events by their name in the specified blocks heights.
func (r *RestProxyHandler) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, nil, err
	}
	defer closer.Close()

	getEventsForHeightRangeRequest := &accessproto.GetEventsForHeightRangeRequest{
		Type:                 eventType,
		StartHeight:          startHeight,
		EndHeight:            endHeight,
		EventEncodingVersion: requiredEventEncodingVersion,
		ExecutionStateQuery:  executionStateQuery(criteria),
	}
	eventsResponse, err := upstream.GetEventsForHeightRange(ctx, getEventsForHeightRangeRequest)
	r.log("upstream", "GetEventsForHeightRange", err)
	if err != nil {
		return nil, nil, err
	}

	res, err := convert.MessagesToBlockEvents(eventsResponse.Results)
	if err != nil {
		return nil, nil, err
	}

	var metadata *accessmodel.ExecutorMetadata
	if rawMetadata := eventsResponse.GetMetadata(); rawMetadata != nil {
		metadata = convert.MessageToExecutorMetadata(rawMetadata.GetExecutorMetadata())
	}

	return res, metadata, nil
}

// GetEventsForBlockIDs returns events by their name in the specified block IDs.
func (r *RestProxyHandler) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error) {
	upstream, closer, err := r.FaultTolerantClient()
	if err != nil {
		return nil, nil, err
	}
	defer closer.Close()

	blockIds := convert.IdentifiersToMessages(blockIDs)

	getEventsForBlockIDsRequest := &accessproto.GetEventsForBlockIDsRequest{
		Type:                 eventType,
		BlockIds:             blockIds,
		EventEncodingVersion: requiredEventEncodingVersion,
		ExecutionStateQuery:  executionStateQuery(criteria),
	}
	eventsResponse, err := upstream.GetEventsForBlockIDs(ctx, getEventsForBlockIDsRequest)
	r.log("upstream", "GetEventsForBlockIDs", err)
	if err != nil {
		return nil, nil, err
	}

	res, err := convert.MessagesToBlockEvents(eventsResponse.Results)
	if err != nil {
		return nil, nil, err
	}

	var metadata *accessmodel.ExecutorMetadata
	if rawMetadata := eventsResponse.GetMetadata(); rawMetadata != nil {
		metadata = convert.MessageToExecutorMetadata(rawMetadata.GetExecutorMetadata())
	}

	return res, metadata, nil
}

// convertError converts a serialized access error formatted as a grpc error returned from the upstream AN,
// to a local access sentinel error.
// if conversion fails, an irrecoverable error is thrown.
func convertError(ctx context.Context, err error, typeName string) error {
	// this is a bit fragile since we're decoding error strings. it's only needed until we add support for execution data on the public network
	switch status.Code(err) {
	case codes.NotFound:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), fmt.Sprintf("data not found for %s: ", typeName)); ok {
			return access.NewDataNotFoundError(typeName, errors.New(sourceErrStr))
		}
	case codes.Internal:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "internal error: "); ok {
			return access.NewInternalError(errors.New(sourceErrStr))
		}
	case codes.OutOfRange:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "out of range: "); ok {
			return access.NewOutOfRangeError(errors.New(sourceErrStr))
		}
	case codes.FailedPrecondition:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "precondition failed: "); ok {
			return access.NewPreconditionFailedError(errors.New(sourceErrStr))
		}
	case codes.InvalidArgument:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "invalid argument: "); ok {
			return access.NewInvalidRequestError(errors.New(sourceErrStr))
		}
	case codes.Canceled:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "request canceled: "); ok {
			return access.NewRequestCanceledError(errors.New(sourceErrStr))
		}
		// it's possible that this came from the client side, so wrap the original error directly.
		return access.NewRequestCanceledError(err)
	case codes.DeadlineExceeded:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "request timed out: "); ok {
			return access.NewRequestTimedOutError(errors.New(sourceErrStr))
		}
		// it's possible that this came from the client side, so wrap the original error directly.
		return access.NewRequestTimedOutError(err)
	case codes.Unavailable:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "service unavailable error: "); ok {
			return access.NewServiceUnavailable(errors.New(sourceErrStr))
		}
		// it's possible that this came from the client side, so wrap the original error directly.
		return access.NewServiceUnavailable(err)
	case codes.ResourceExhausted:
		if sourceErrStr, ok := splitOnPrefix(err.Error(), "resource exhausted error: "); ok {
			return access.NewResourceExhausted(errors.New(sourceErrStr))
		}
		// it's possible that this came from the client side, so wrap the original error directly.
		return access.NewResourceExhausted(err)
	}

	// all methods MUST return an access sentinel error. if we couldn't successfully convert the error,
	// then there is a bug. throw an irrecoverable exception.
	return access.RequireNoError(ctx, fmt.Errorf("failed to convert upstream error: %w", err))
}

func splitOnPrefix(original, prefix string) (string, bool) {
	parts := strings.Split(original, prefix)
	if len(parts) == 2 {
		return parts[1], true
	}
	return "", false
}

// executionStateQuery constructs an ExecutionStateQuery protobuf message from
// the provided optimistic sync Criteria.
// The IncludeExecutorMetadata field is set to true, allowing metadata to be included
// in the response if needed.
func executionStateQuery(criteria optimistic_sync.Criteria) *entities.ExecutionStateQuery {
	return &entities.ExecutionStateQuery{
		AgreeingExecutorsCount:  uint64(criteria.AgreeingExecutorsCount),
		RequiredExecutorIds:     convert.IdentifiersToMessages(criteria.RequiredExecutors),
		IncludeExecutorMetadata: true,
	}
}
