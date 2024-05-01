package apiproxy

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessflow "github.com/onflow/flow-go/access"
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
	local    *accessflow.Handler
	useIndex bool
}

type Params struct {
	Log      zerolog.Logger
	Metrics  *metrics.ObserverCollector
	Upstream *FlowAccessAPIForwarder
	Local    *accessflow.Handler
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
func (h *FlowAccessAPIRouter) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	h.log(LocalApiService, "Ping", nil)
	return &access.PingResponse{}, nil
}

func (h *FlowAccessAPIRouter) GetNodeVersionInfo(ctx context.Context, request *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	res, err := h.local.GetNodeVersionInfo(ctx, request)
	h.log(LocalApiService, "GetNodeVersionInfo", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetLatestBlockHeader(context, req)
	h.log(LocalApiService, "GetLatestBlockHeader", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetBlockHeaderByID(context, req)
	h.log(LocalApiService, "GetBlockHeaderByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetBlockHeaderByHeight(context, req)
	h.log(LocalApiService, "GetBlockHeaderByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetLatestBlock(context, req)
	h.log(LocalApiService, "GetLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetBlockByID(context, req)
	h.log(LocalApiService, "GetBlockByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetBlockByHeight(context, req)
	h.log(LocalApiService, "GetBlockByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetCollectionByID(context, req)
		h.log(LocalApiService, "GetCollectionByID", err)
		return res, err
	}

	res, err := h.upstream.GetCollectionByID(context, req)
	h.log(UpstreamApiService, "GetCollectionByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	res, err := h.upstream.SendTransaction(context, req)
	h.log(UpstreamApiService, "SendTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetTransaction(context, req)
		h.log(LocalApiService, "GetTransaction", err)
		return res, err
	}

	res, err := h.upstream.GetTransaction(context, req)
	h.log(UpstreamApiService, "GetTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	//TODO: add implementation for transaction error message before adding local impl

	res, err := h.upstream.GetTransactionResult(context, req)
	h.log(UpstreamApiService, "GetTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	//TODO: add implementation for transaction error message before adding local impl

	res, err := h.upstream.GetTransactionResultsByBlockID(context, req)
	h.log(UpstreamApiService, "GetTransactionResultsByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetTransactionsByBlockID(context, req)
		h.log(LocalApiService, "GetTransactionsByBlockID", err)
		return res, err
	}

	res, err := h.upstream.GetTransactionsByBlockID(context, req)
	h.log(UpstreamApiService, "GetTransactionsByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	//TODO: add implementation for transaction error message before adding local impl

	res, err := h.upstream.GetTransactionResultByIndex(context, req)
	h.log(UpstreamApiService, "GetTransactionResultByIndex", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetSystemTransaction(context context.Context, req *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	if h.useIndex {
		res, err := h.local.GetSystemTransaction(context, req)
		h.log(LocalApiService, "GetSystemTransaction", err)
		return res, err
	}

	res, err := h.upstream.GetSystemTransaction(context, req)
	h.log(UpstreamApiService, "GetSystemTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetSystemTransactionResult(context context.Context, req *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	res, err := h.upstream.GetSystemTransactionResult(context, req)
	h.log(UpstreamApiService, "GetSystemTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccount(context, req)
		h.log(LocalApiService, "GetAccount", err)
		return res, err
	}

	res, err := h.upstream.GetAccount(context, req)
	h.log(UpstreamApiService, "GetAccount", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountAtLatestBlock(context, req)
		h.log(LocalApiService, "GetAccountAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.GetAccountAtLatestBlock(context, req)
	h.log(UpstreamApiService, "GetAccountAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	if h.useIndex {
		res, err := h.local.GetAccountAtBlockHeight(context, req)
		h.log(LocalApiService, "GetAccountAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.GetAccountAtBlockHeight(context, req)
	h.log(UpstreamApiService, "GetAccountAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	if h.useIndex {
		res, err := h.local.ExecuteScriptAtLatestBlock(context, req)
		h.log(LocalApiService, "ExecuteScriptAtLatestBlock", err)
		return res, err
	}

	res, err := h.upstream.ExecuteScriptAtLatestBlock(context, req)
	h.log(UpstreamApiService, "ExecuteScriptAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	if h.useIndex {
		res, err := h.local.ExecuteScriptAtBlockID(context, req)
		h.log(LocalApiService, "ExecuteScriptAtBlockID", err)
		return res, err
	}

	res, err := h.upstream.ExecuteScriptAtBlockID(context, req)
	h.log(UpstreamApiService, "ExecuteScriptAtBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	if h.useIndex {
		res, err := h.local.ExecuteScriptAtBlockHeight(context, req)
		h.log(LocalApiService, "ExecuteScriptAtBlockHeight", err)
		return res, err
	}

	res, err := h.upstream.ExecuteScriptAtBlockHeight(context, req)
	h.log(UpstreamApiService, "ExecuteScriptAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetEventsForHeightRange(context, req)
		h.log(LocalApiService, "GetEventsForHeightRange", err)
		return res, err
	}

	res, err := h.upstream.GetEventsForHeightRange(context, req)
	h.log(UpstreamApiService, "GetEventsForHeightRange", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetEventsForBlockIDs(context, req)
		h.log(LocalApiService, "GetEventsForBlockIDs", err)
		return res, err
	}

	res, err := h.upstream.GetEventsForBlockIDs(context, req)
	h.log(UpstreamApiService, "GetEventsForBlockIDs", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	res, err := h.local.GetNetworkParameters(context, req)
	h.log(LocalApiService, "GetNetworkParameters", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetLatestProtocolStateSnapshot(context, req)
	h.log(LocalApiService, "GetLatestProtocolStateSnapshot", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetProtocolStateSnapshotByBlockID(context context.Context, req *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetProtocolStateSnapshotByBlockID(context, req)
	h.log(LocalApiService, "GetProtocolStateSnapshotByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetProtocolStateSnapshotByHeight(context context.Context, req *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetProtocolStateSnapshotByHeight(context, req)
	h.log(LocalApiService, "GetProtocolStateSnapshotByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	res, err := h.upstream.GetExecutionResultForBlockID(context, req)
	h.log(UpstreamApiService, "GetExecutionResultForBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetExecutionResultByID(context context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	if h.useIndex {
		res, err := h.local.GetExecutionResultByID(context, req)
		h.log(LocalApiService, "GetExecutionResultByID", err)
		return res, err
	}

	res, err := h.upstream.GetExecutionResultByID(context, req)
	h.log(UpstreamApiService, "GetExecutionResultByID", err)
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
	//SendAndSubscribeTransactionStatuses is not implemented for observer yet
	return status.Errorf(codes.Unimplemented, "method SendAndSubscribeTransactionStatuses not implemented")
}

// FlowAccessAPIForwarder forwards all requests to a set of upstream access nodes or observers
type FlowAccessAPIForwarder struct {
	*forwarder.Forwarder
}

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
func (h *FlowAccessAPIForwarder) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.Ping(context, req)
}

func (h *FlowAccessAPIForwarder) GetNodeVersionInfo(context context.Context, req *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetNodeVersionInfo(context, req)
}

func (h *FlowAccessAPIForwarder) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetLatestBlockHeader(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockHeaderByID(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockHeaderByHeight(context, req)
}

func (h *FlowAccessAPIForwarder) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetLatestBlock(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockByID(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetBlockByHeight(context, req)
}

func (h *FlowAccessAPIForwarder) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetCollectionByID(context, req)
}

func (h *FlowAccessAPIForwarder) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.SendTransaction(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransaction(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionResult(context, req)
}

func (h *FlowAccessAPIForwarder) GetSystemTransaction(context context.Context, req *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetSystemTransaction(context, req)
}

func (h *FlowAccessAPIForwarder) GetSystemTransactionResult(context context.Context, req *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetSystemTransactionResult(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionResultByIndex(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionResultsByBlockID(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetTransactionsByBlockID(context, req)
}

func (h *FlowAccessAPIForwarder) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccount(context, req)
}

func (h *FlowAccessAPIForwarder) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountAtLatestBlock(context, req)
}

func (h *FlowAccessAPIForwarder) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetAccountAtBlockHeight(context, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.ExecuteScriptAtLatestBlock(context, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.ExecuteScriptAtBlockID(context, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.ExecuteScriptAtBlockHeight(context, req)
}

func (h *FlowAccessAPIForwarder) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetEventsForHeightRange(context, req)
}

func (h *FlowAccessAPIForwarder) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetEventsForBlockIDs(context, req)
}

func (h *FlowAccessAPIForwarder) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetNetworkParameters(context, req)
}

func (h *FlowAccessAPIForwarder) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetLatestProtocolStateSnapshot(context, req)
}

func (h *FlowAccessAPIForwarder) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetExecutionResultForBlockID(context, req)
}

func (h *FlowAccessAPIForwarder) GetExecutionResultByID(context context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	// This is a passthrough request
	upstream, closer, err := h.FaultTolerantClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return upstream.GetExecutionResultByID(context, req)
}
