package apiproxy

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/grpc/forwarder"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

const (
	LocalApiService    = "local"
	UpstreamApiService = "upstream"
)

// FlowAccessAPIRouter is a structure that represents the routing proxy algorithm.
// It splits requests between a local and a remote API service.
type FlowAccessAPIRouter struct {
	logger   zerolog.Logger
	metrics  *metrics.ObserverCollector
	upstream *FlowAccessAPIForwarder
	local    *rpc.Handler
	useIndex bool
}

var _ access.AccessAPIServer = (*FlowAccessAPIRouter)(nil)

type Params struct {
	Log      zerolog.Logger
	Metrics  *metrics.ObserverCollector
	Upstream *FlowAccessAPIForwarder
	Local    *rpc.Handler
	UseIndex bool
}

// NewFlowAccessAPIRouter creates FlowAccessAPIRouter instance
func NewFlowAccessAPIRouter(params Params) *FlowAccessAPIRouter {
	h := &FlowAccessAPIRouter{
		logger:   params.Log,
		metrics:  params.Metrics,
		upstream: params.Upstream,
		local:    params.Local,
		useIndex: params.UseIndex,
	}

	return h
}

func (h *FlowAccessAPIRouter) log(handler, rpc string, err error) {
	code := status.Code(err)
	h.metrics.RecordRPC(handler, rpc, code)

	logger := h.logger.With().
		Str("handler", handler).
		Str("grpc_method", rpc).
		Str("grpc_code", code.String()).
		Logger()

	if err != nil {
		logger.Error().Err(err).Msg("request failed")
		return
	}

	logger.Info().Msg("request succeeded")
}

// Ping pings the service. It is special in the sense that it responds successful,
// only if all underlying services are ready.
func (h *FlowAccessAPIRouter) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	h.log(LocalApiService, "Ping", nil)
	return &access.PingResponse{}, nil
}

func (h *FlowAccessAPIRouter) GetNodeVersionInfo(ctx context.Context, request *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	res, err := h.local.GetNodeVersionInfo(ctx, request)
	h.log(LocalApiService, "GetNodeVersionInfo", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestBlockHeader(ctx context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetLatestBlockHeader(ctx, req)
	h.log(LocalApiService, "GetLatestBlockHeader", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByID(ctx context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetBlockHeaderByID(ctx, req)
	h.log(LocalApiService, "GetBlockHeaderByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByHeight(ctx context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetBlockHeaderByHeight(ctx, req)
	h.log(LocalApiService, "GetBlockHeaderByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestBlock(ctx context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetLatestBlock(ctx, req)
	h.log(LocalApiService, "GetLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockByID(ctx context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetBlockByID(ctx, req)
	h.log(LocalApiService, "GetBlockByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockByHeight(ctx context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetBlockByHeight(ctx, req)
	h.log(LocalApiService, "GetBlockByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetCollectionByID(ctx context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetCollectionByID(ctx, req)
		h.log(LocalApiService, "GetCollectionByID", err)
		return res, err
	}

	res, err := h.upstream.GetCollectionByID(ctx, req)
	h.log(UpstreamApiService, "GetCollectionByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetFullCollectionByID(ctx context.Context, req *access.GetFullCollectionByIDRequest) (*access.FullCollectionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetFullCollectionByID(ctx, req)
		h.log(LocalApiService, "GetFullCollectionByID", err)
		return res, err
	}

	res, err := h.upstream.GetFullCollectionByID(ctx, req)
	h.log(UpstreamApiService, "GetFullCollectionByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) SendTransaction(ctx context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	res, err := h.upstream.SendTransaction(ctx, req)
	h.log(UpstreamApiService, "SendTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransaction(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetTransaction(ctx, req)
		h.log(LocalApiService, "GetTransaction", err)
		return res, err
	}

	res, err := h.upstream.GetTransaction(ctx, req)
	h.log(UpstreamApiService, "GetTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResult(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	// TODO: add implementation for transaction error message before adding local impl

	res, err := h.upstream.GetTransactionResult(ctx, req)
	h.log(UpstreamApiService, "GetTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResultsByBlockID(ctx context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	// TODO: add implementation for transaction error message before adding local impl

	res, err := h.upstream.GetTransactionResultsByBlockID(ctx, req)
	h.log(UpstreamApiService, "GetTransactionResultsByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionsByBlockID(ctx context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetTransactionsByBlockID(ctx, req)
		h.log(LocalApiService, "GetTransactionsByBlockID", err)
		return res, err
	}

	res, err := h.upstream.GetTransactionsByBlockID(ctx, req)
	h.log(UpstreamApiService, "GetTransactionsByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResultByIndex(ctx context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	// TODO: add implementation for transaction error message before adding local impl

	res, err := h.upstream.GetTransactionResultByIndex(ctx, req)
	h.log(UpstreamApiService, "GetTransactionResultByIndex", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetSystemTransaction(ctx context.Context, req *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetSystemTransaction(ctx, req)
		h.log(LocalApiService, "GetSystemTransaction", err)
		return res, err
	}

	res, err := h.upstream.GetSystemTransaction(ctx, req)
	h.log(UpstreamApiService, "GetSystemTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetSystemTransactionResult(ctx context.Context, req *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	res, err := h.upstream.GetSystemTransactionResult(ctx, req)
	h.log(UpstreamApiService, "GetSystemTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetScheduledTransaction(ctx context.Context, req *access.GetScheduledTransactionRequest) (*access.TransactionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetScheduledTransaction(ctx, req)
		h.log(LocalApiService, "GetScheduledTransaction", err)
		return res, err
	}

	res, err := h.upstream.GetScheduledTransaction(ctx, req)
	h.log(UpstreamApiService, "GetScheduledTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetScheduledTransactionResult(ctx context.Context, req *access.GetScheduledTransactionResultRequest) (*access.TransactionResultResponse, error) {
	if h.useIndex {
		res, err := h.local.GetScheduledTransactionResult(ctx, req)
		h.log(LocalApiService, "GetScheduledTransactionResult", err)
		return res, err
	}

	res, err := h.upstream.GetScheduledTransactionResult(ctx, req)
	h.log(UpstreamApiService, "GetScheduledTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccount(ctx context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccount(ctx, req)
		h.log(LocalApiService, "GetAccount", err)
		return res, err
	}

	res, err := h.upstream.GetAccount(ctx, req)
	h.log(UpstreamApiService, "GetAccount", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountAtLatestBlock(ctx context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountAtLatestBlock(ctx, req)
		h.log(LocalApiService, "GetAccountAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.GetAccountAtLatestBlock(ctx, req)
	h.log(UpstreamApiService, "GetAccountAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountAtBlockHeight(ctx context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountAtBlockHeight(ctx, req)
		h.log(LocalApiService, "GetAccountAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.GetAccountAtBlockHeight(ctx, req)
	h.log(UpstreamApiService, "GetAccountAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountBalanceAtLatestBlock(ctx context.Context, req *access.GetAccountBalanceAtLatestBlockRequest) (*access.AccountBalanceResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountBalanceAtLatestBlock(ctx, req)
		h.log(LocalApiService, "GetAccountBalanceAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.GetAccountBalanceAtLatestBlock(ctx, req)
	h.log(UpstreamApiService, "GetAccountBalanceAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountBalanceAtBlockHeight(ctx context.Context, req *access.GetAccountBalanceAtBlockHeightRequest) (*access.AccountBalanceResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountBalanceAtBlockHeight(ctx, req)
		h.log(LocalApiService, "GetAccountBalanceAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.GetAccountBalanceAtBlockHeight(ctx, req)
	h.log(UpstreamApiService, "GetAccountBalanceAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountKeyAtLatestBlock(ctx context.Context, req *access.GetAccountKeyAtLatestBlockRequest) (*access.AccountKeyResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountKeyAtLatestBlock(ctx, req)
		h.log(LocalApiService, "GetAccountKeyAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.GetAccountKeyAtLatestBlock(ctx, req)
	h.log(UpstreamApiService, "GetAccountKeyAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountKeysAtLatestBlock(ctx context.Context, req *access.GetAccountKeysAtLatestBlockRequest) (*access.AccountKeysResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountKeysAtLatestBlock(ctx, req)
		h.log(LocalApiService, "GetAccountKeysAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.GetAccountKeysAtLatestBlock(ctx, req)
	h.log(UpstreamApiService, "GetAccountKeysAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountKeyAtBlockHeight(ctx context.Context, req *access.GetAccountKeyAtBlockHeightRequest) (*access.AccountKeyResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountKeyAtBlockHeight(ctx, req)
		h.log(LocalApiService, "GetAccountKeyAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.GetAccountKeyAtBlockHeight(ctx, req)
	h.log(UpstreamApiService, "GetAccountKeyAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountKeysAtBlockHeight(ctx context.Context, req *access.GetAccountKeysAtBlockHeightRequest) (*access.AccountKeysResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountKeysAtBlockHeight(ctx, req)
		h.log(LocalApiService, "GetAccountKeysAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.GetAccountKeysAtBlockHeight(ctx, req)
	h.log(UpstreamApiService, "GetAccountKeysAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtLatestBlock(ctx context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	if h.useIndex {
		res, err := h.local.ExecuteScriptAtLatestBlock(ctx, req)
		h.log(LocalApiService, "ExecuteScriptAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.ExecuteScriptAtLatestBlock(ctx, req)
	h.log(UpstreamApiService, "ExecuteScriptAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockID(ctx context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	if h.useIndex {
		res, err := h.local.ExecuteScriptAtBlockID(ctx, req)
		h.log(LocalApiService, "ExecuteScriptAtBlockID", err)
		return res, err
	}

	res, err := h.upstream.ExecuteScriptAtBlockID(ctx, req)
	h.log(UpstreamApiService, "ExecuteScriptAtBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockHeight(ctx context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	if h.useIndex {
		res, err := h.local.ExecuteScriptAtBlockHeight(ctx, req)
		h.log(LocalApiService, "ExecuteScriptAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.ExecuteScriptAtBlockHeight(ctx, req)
	h.log(UpstreamApiService, "ExecuteScriptAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetEventsForHeightRange(ctx context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetEventsForHeightRange(ctx, req)
		h.log(LocalApiService, "GetEventsForHeightRange", err)
		return res, err
	}

	res, err := h.upstream.GetEventsForHeightRange(ctx, req)
	h.log(UpstreamApiService, "GetEventsForHeightRange", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetEventsForBlockIDs(ctx context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetEventsForBlockIDs(ctx, req)
		h.log(LocalApiService, "GetEventsForBlockIDs", err)
		return res, err
	}

	res, err := h.upstream.GetEventsForBlockIDs(ctx, req)
	h.log(UpstreamApiService, "GetEventsForBlockIDs", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetNetworkParameters(ctx context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	res, err := h.local.GetNetworkParameters(ctx, req)
	h.log(LocalApiService, "GetNetworkParameters", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestProtocolStateSnapshot(ctx context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetLatestProtocolStateSnapshot(ctx, req)
	h.log(LocalApiService, "GetLatestProtocolStateSnapshot", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetProtocolStateSnapshotByBlockID(ctx context.Context, req *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetProtocolStateSnapshotByBlockID(ctx, req)
	h.log(LocalApiService, "GetProtocolStateSnapshotByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetProtocolStateSnapshotByHeight(ctx context.Context, req *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetProtocolStateSnapshotByHeight(ctx, req)
	h.log(LocalApiService, "GetProtocolStateSnapshotByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetExecutionResultForBlockID(ctx context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	res, err := h.local.GetExecutionResultForBlockID(ctx, req)
	h.log(LocalApiService, "GetExecutionResultForBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetExecutionResultByID(ctx context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	res, err := h.local.GetExecutionResultByID(ctx, req)
	h.log(LocalApiService, "GetExecutionResultByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) SubscribeBlocksFromStartBlockID(req *access.SubscribeBlocksFromStartBlockIDRequest, server access.AccessAPI_SubscribeBlocksFromStartBlockIDServer) error {
	err := h.local.SubscribeBlocksFromStartBlockID(req, server)
	h.log(LocalApiService, "SubscribeBlocksFromStartBlockID", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlocksFromStartHeight(req *access.SubscribeBlocksFromStartHeightRequest, server access.AccessAPI_SubscribeBlocksFromStartHeightServer) error {
	err := h.local.SubscribeBlocksFromStartHeight(req, server)
	h.log(LocalApiService, "SubscribeBlocksFromStartHeight", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlocksFromLatest(req *access.SubscribeBlocksFromLatestRequest, server access.AccessAPI_SubscribeBlocksFromLatestServer) error {
	err := h.local.SubscribeBlocksFromLatest(req, server)
	h.log(LocalApiService, "SubscribeBlocksFromLatest", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlockHeadersFromStartBlockID(req *access.SubscribeBlockHeadersFromStartBlockIDRequest, server access.AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer) error {
	err := h.local.SubscribeBlockHeadersFromStartBlockID(req, server)
	h.log(LocalApiService, "SubscribeBlockHeadersFromStartBlockID", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlockHeadersFromStartHeight(req *access.SubscribeBlockHeadersFromStartHeightRequest, server access.AccessAPI_SubscribeBlockHeadersFromStartHeightServer) error {
	err := h.local.SubscribeBlockHeadersFromStartHeight(req, server)
	h.log(LocalApiService, "SubscribeBlockHeadersFromStartHeight", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlockHeadersFromLatest(req *access.SubscribeBlockHeadersFromLatestRequest, server access.AccessAPI_SubscribeBlockHeadersFromLatestServer) error {
	err := h.local.SubscribeBlockHeadersFromLatest(req, server)
	h.log(LocalApiService, "SubscribeBlockHeadersFromLatest", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlockDigestsFromStartBlockID(req *access.SubscribeBlockDigestsFromStartBlockIDRequest, server access.AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer) error {
	err := h.local.SubscribeBlockDigestsFromStartBlockID(req, server)
	h.log(LocalApiService, "SubscribeBlockDigestsFromStartBlockID", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlockDigestsFromStartHeight(req *access.SubscribeBlockDigestsFromStartHeightRequest, server access.AccessAPI_SubscribeBlockDigestsFromStartHeightServer) error {
	err := h.local.SubscribeBlockDigestsFromStartHeight(req, server)
	h.log(LocalApiService, "SubscribeBlockDigestsFromStartHeight", err)
	return err
}

func (h *FlowAccessAPIRouter) SubscribeBlockDigestsFromLatest(req *access.SubscribeBlockDigestsFromLatestRequest, server access.AccessAPI_SubscribeBlockDigestsFromLatestServer) error {
	err := h.local.SubscribeBlockDigestsFromLatest(req, server)
	h.log(LocalApiService, "SubscribeBlockDigestsFromLatest", err)
	return err
}

func (h *FlowAccessAPIRouter) SendAndSubscribeTransactionStatuses(req *access.SendAndSubscribeTransactionStatusesRequest, server access.AccessAPI_SendAndSubscribeTransactionStatusesServer) error {
	// SendAndSubscribeTransactionStatuses is not implemented for observer yet
	return status.Errorf(codes.Unimplemented, "method SendAndSubscribeTransactionStatuses not implemented")
}

// FlowAccessAPIForwarder forwards all requests to a set of upstream access nodes or observers
type FlowAccessAPIForwarder struct {
	*forwarder.Forwarder
}

var _ access.AccessAPIServer = (*FlowAccessAPIForwarder)(nil)

func NewFlowAccessAPIForwarder(identities flow.IdentitySkeletonList, connectionFactory connection.ConnectionFactory) (*FlowAccessAPIForwarder, error) {
	forwarder, err := forwarder.NewForwarder(identities, connectionFactory)
	if err != nil {
		return nil, err
	}

	return &FlowAccessAPIForwarder{
		Forwarder: forwarder,
	}, nil
}

// Ping pings the service. It is special in the sense that it responds successful,
// only if all underlying services are ready.
func (h *FlowAccessAPIForwarder) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.Ping(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetNodeVersionInfo(ctx context.Context, req *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetNodeVersionInfo(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetLatestBlockHeader(ctx context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetLatestBlockHeader(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetBlockHeaderByID(ctx context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockHeaderByID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetBlockHeaderByHeight(ctx context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockHeaderByHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetLatestBlock(ctx context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetLatestBlock(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetBlockByID(ctx context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockByID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetBlockByHeight(ctx context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockByHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetCollectionByID(ctx context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetCollectionByID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetFullCollectionByID(ctx context.Context, req *access.GetFullCollectionByIDRequest) (*access.FullCollectionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetFullCollectionByID(ctx, req)
}

func (h *FlowAccessAPIForwarder) SendTransaction(ctx context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.SendTransaction(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetTransaction(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransaction(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResult(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionResult(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetSystemTransaction(ctx context.Context, req *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetSystemTransaction(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetScheduledTransaction(ctx context.Context, req *access.GetScheduledTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetScheduledTransaction(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetScheduledTransactionResult(ctx context.Context, req *access.GetScheduledTransactionResultRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetScheduledTransactionResult(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetSystemTransactionResult(ctx context.Context, req *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetSystemTransactionResult(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResultByIndex(ctx context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionResultByIndex(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResultsByBlockID(ctx context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionResultsByBlockID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionsByBlockID(ctx context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionsByBlockID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccount(ctx context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccount(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountAtLatestBlock(ctx context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountAtLatestBlock(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountAtBlockHeight(ctx context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountAtBlockHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountBalanceAtLatestBlock(ctx context.Context, req *access.GetAccountBalanceAtLatestBlockRequest) (*access.AccountBalanceResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountBalanceAtLatestBlock(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountBalanceAtBlockHeight(ctx context.Context, req *access.GetAccountBalanceAtBlockHeightRequest) (*access.AccountBalanceResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountBalanceAtBlockHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountKeyAtLatestBlock(ctx context.Context, req *access.GetAccountKeyAtLatestBlockRequest) (*access.AccountKeyResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountKeyAtLatestBlock(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountKeysAtLatestBlock(ctx context.Context, req *access.GetAccountKeysAtLatestBlockRequest) (*access.AccountKeysResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountKeysAtLatestBlock(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountKeyAtBlockHeight(ctx context.Context, req *access.GetAccountKeyAtBlockHeightRequest) (*access.AccountKeyResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountKeyAtBlockHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetAccountKeysAtBlockHeight(ctx context.Context, req *access.GetAccountKeysAtBlockHeightRequest) (*access.AccountKeysResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountKeysAtBlockHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtLatestBlock(ctx context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.ExecuteScriptAtLatestBlock(ctx, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtBlockID(ctx context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.ExecuteScriptAtBlockID(ctx, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtBlockHeight(ctx context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.ExecuteScriptAtBlockHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetEventsForHeightRange(ctx context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetEventsForHeightRange(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetEventsForBlockIDs(ctx context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetEventsForBlockIDs(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetNetworkParameters(ctx context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetNetworkParameters(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetLatestProtocolStateSnapshot(ctx context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetLatestProtocolStateSnapshot(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetProtocolStateSnapshotByBlockID(ctx context.Context, req *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetProtocolStateSnapshotByBlockID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetProtocolStateSnapshotByHeight(ctx context.Context, req *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetProtocolStateSnapshotByHeight(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetExecutionResultForBlockID(ctx context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetExecutionResultForBlockID(ctx, req)
}

func (h *FlowAccessAPIForwarder) GetExecutionResultByID(ctx context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetExecutionResultByID(ctx, req)
}

func (h *FlowAccessAPIForwarder) SendAndSubscribeTransactionStatuses(req *access.SendAndSubscribeTransactionStatusesRequest, server access.AccessAPI_SendAndSubscribeTransactionStatusesServer) error {
	return status.Errorf(codes.Unimplemented, "method SendAndSubscribeTransactionStatuses not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlockDigestsFromLatest(req *access.SubscribeBlockDigestsFromLatestRequest, server access.AccessAPI_SubscribeBlockDigestsFromLatestServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlockDigestsFromLatest not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlockDigestsFromStartBlockID(req *access.SubscribeBlockDigestsFromStartBlockIDRequest, server access.AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlockDigestsFromStartBlockID not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlockDigestsFromStartHeight(req *access.SubscribeBlockDigestsFromStartHeightRequest, server access.AccessAPI_SubscribeBlockDigestsFromStartHeightServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlockDigestsFromStartHeight not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlockHeadersFromLatest(req *access.SubscribeBlockHeadersFromLatestRequest, server access.AccessAPI_SubscribeBlockHeadersFromLatestServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlockHeadersFromLatest not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlockHeadersFromStartBlockID(req *access.SubscribeBlockHeadersFromStartBlockIDRequest, server access.AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlockHeadersFromStartBlockID not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlockHeadersFromStartHeight(req *access.SubscribeBlockHeadersFromStartHeightRequest, server access.AccessAPI_SubscribeBlockHeadersFromStartHeightServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlockHeadersFromStartHeight not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlocksFromLatest(req *access.SubscribeBlocksFromLatestRequest, server access.AccessAPI_SubscribeBlocksFromLatestServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlocksFromLatest not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlocksFromStartBlockID(req *access.SubscribeBlocksFromStartBlockIDRequest, server access.AccessAPI_SubscribeBlocksFromStartBlockIDServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlocksFromStartBlockID not implemented")
}

func (h *FlowAccessAPIForwarder) SubscribeBlocksFromStartHeight(req *access.SubscribeBlocksFromStartHeightRequest, server access.AccessAPI_SubscribeBlocksFromStartHeightServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeBlocksFromStartHeight not implemented")
}
