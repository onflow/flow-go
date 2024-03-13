package apiproxy

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/status"

	accessflow "github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/grpc/forwarder"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	stateProtocol "github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// FlowAccessAPIRouter is a structure that represents the routing proxy algorithm.
// It splits requests between a local and a remote API service.
type FlowAccessAPIRouter struct {
	logger   zerolog.Logger
	metrics  *metrics.ObserverCollector
	upstream *FlowAccessAPIForwarder
	//observer *protocol.Handler

	local    *accessflow.Handler
	useIndex bool
}

type Params struct {
	Log         zerolog.Logger
	State       stateProtocol.State
	Blocks      storage.Blocks
	Headers     storage.Headers
	RootChainID flow.ChainID
	Metrics     *metrics.ObserverCollector
	Upstream    *FlowAccessAPIForwarder
	Local       *accessflow.Handler
	UseIndex    bool
}

// NewFlowAccessAPIRouter creates FlowAccessAPIRouter instance
func NewFlowAccessAPIRouter(params Params) *FlowAccessAPIRouter {
	h := &FlowAccessAPIRouter{
		logger:   params.Log,
		metrics:  params.Metrics,
		upstream: params.Upstream,
		//observer: protocol.NewHandler(protocol.New(
		//	params.State,
		//	params.Blocks,
		//	params.Headers,
		//	backend.NewNetworkAPI(
		//		params.State,
		//		params.RootChainID,
		//		params.Headers,
		//		backend.DefaultSnapshotHistoryLimit,
		//	),
		//)),
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
	h.log("local", "Ping", nil)
	return &access.PingResponse{}, nil
}

func (h *FlowAccessAPIRouter) GetNodeVersionInfo(ctx context.Context, request *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	res, err := h.local.GetNodeVersionInfo(ctx, request)
	h.log("local", "GetNodeVersionInfo", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetLatestBlockHeader(context, req)
	h.log("local", "GetLatestBlockHeader", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetBlockHeaderByID(context, req)
	h.log("local", "GetBlockHeaderByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	res, err := h.local.GetBlockHeaderByHeight(context, req)
	h.log("local", "GetBlockHeaderByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetLatestBlock(context, req)
	h.log("local", "GetLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetBlockByID(context, req)
	h.log("local", "GetBlockByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	res, err := h.local.GetBlockByHeight(context, req)
	h.log("local", "GetBlockByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	res, err := h.upstream.GetCollectionByID(context, req)
	h.log("upstream", "GetCollectionByID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	res, err := h.upstream.SendTransaction(context, req)
	h.log("upstream", "SendTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	res, err := h.upstream.GetTransaction(context, req)
	h.log("upstream", "GetTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	//if h.useIndex {
	//	// TODO: add impl for tx errors
	//	res, err := h.local.GetTransactionResult(context, req)
	//	h.log("local", "GetTransactionResult", err)
	//	return res, err
	//}

	res, err := h.upstream.GetTransactionResult(context, req)
	h.log("upstream", "GetTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	res, err := h.upstream.GetTransactionResultsByBlockID(context, req)
	h.log("upstream", "GetTransactionResultsByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	res, err := h.upstream.GetTransactionsByBlockID(context, req)
	h.log("upstream", "GetTransactionsByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	res, err := h.upstream.GetTransactionResultByIndex(context, req)
	h.log("upstream", "GetTransactionResultByIndex", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetSystemTransaction(context context.Context, req *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	res, err := h.upstream.GetSystemTransaction(context, req)
	h.log("upstream", "GetSystemTransaction", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetSystemTransactionResult(context context.Context, req *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	res, err := h.upstream.GetSystemTransactionResult(context, req)
	h.log("upstream", "GetSystemTransactionResult", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	res, err := h.upstream.GetAccount(context, req)
	h.log("upstream", "GetAccount", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	res, err := h.upstream.GetAccountAtLatestBlock(context, req)
	h.log("upstream", "GetAccountAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	res, err := h.upstream.GetAccountAtBlockHeight(context, req)
	h.log("upstream", "GetAccountAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	res, err := h.upstream.ExecuteScriptAtLatestBlock(context, req)
	h.log("upstream", "ExecuteScriptAtLatestBlock", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	res, err := h.upstream.ExecuteScriptAtBlockID(context, req)
	h.log("upstream", "ExecuteScriptAtBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	res, err := h.upstream.ExecuteScriptAtBlockHeight(context, req)
	h.log("upstream", "ExecuteScriptAtBlockHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	if h.useIndex {
		res, err := h.local.GetEventsForHeightRange(context, req)
		h.log("local", "GetTransactionResult", err)
		return res, err
	}

	res, err := h.upstream.GetEventsForHeightRange(context, req)
	h.log("upstream", "GetEventsForHeightRange", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {

	if h.useIndex {
		res, err := h.local.GetEventsForBlockIDs(context, req)
		h.log("local", "GetTransactionResult", err)
		return res, err
	}

	res, err := h.upstream.GetEventsForBlockIDs(context, req)
	h.log("upstream", "GetEventsForBlockIDs", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	res, err := h.local.GetNetworkParameters(context, req)
	h.log("local", "GetNetworkParameters", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetLatestProtocolStateSnapshot(context, req)
	h.log("local", "GetLatestProtocolStateSnapshot", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetProtocolStateSnapshotByBlockID(context context.Context, req *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetProtocolStateSnapshotByBlockID(context, req)
	h.log("local", "GetProtocolStateSnapshotByBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetProtocolStateSnapshotByHeight(context context.Context, req *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	res, err := h.local.GetProtocolStateSnapshotByHeight(context, req)
	h.log("local", "GetProtocolStateSnapshotByHeight", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	res, err := h.upstream.GetExecutionResultForBlockID(context, req)
	h.log("upstream", "GetExecutionResultForBlockID", err)
	return res, err
}

func (h *FlowAccessAPIRouter) GetExecutionResultByID(context context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	res, err := h.upstream.GetExecutionResultByID(context, req)
	h.log("upstream", "GetExecutionResultByID", err)
	return res, err
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

//// FlowAccessAPILocalForwarder forwards all requests to a backend with IndexQueryModeLocalOnly mode
//type FlowAccessAPILocalForwarder struct {
//	FlowAccessAPIForwarder
//
//	accessBackend *backend.Backend
//	nodeId        flow.Identifier
//	state         stateProtocol.State
//}
//
//var _ FlowAccessAPIForwarderInterface = (*FlowAccessAPILocalForwarder)(nil)
//
//func NewFlowAccessAPILocalForwarder(accessBackend *backend.Backend, nodeId flow.Identifier, state stateProtocol.State, identities flow.IdentitySkeletonList, connectionFactory connection.ConnectionFactory) (*FlowAccessAPILocalForwarder, error) {
//	forwarder, err := forwarder.NewForwarder(identities, connectionFactory)
//	if err != nil {
//		return nil, err
//	}
//
//	return &FlowAccessAPILocalForwarder{
//		FlowAccessAPIForwarder: FlowAccessAPIForwarder{
//			forwarder,
//		},
//		accessBackend: accessBackend,
//		nodeId:        nodeId,
//		state:         state,
//	}, nil
//}
//
//func (h *FlowAccessAPILocalForwarder) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
//	transactionID, err := rpcConvert.TransactionID(req.GetId())
//	if err != nil {
//		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction id: %v", err)
//	}
//
//	blockId := flow.ZeroID
//	requestBlockId := req.GetBlockId()
//	if requestBlockId != nil {
//		blockId, err = rpcConvert.BlockID(requestBlockId)
//		if err != nil {
//			return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
//		}
//	}
//
//	collectionId := flow.ZeroID
//	requestCollectionId := req.GetCollectionId()
//	if requestCollectionId != nil {
//		collectionId, err = rpcConvert.CollectionID(requestCollectionId)
//		if err != nil {
//			return nil, status.Errorf(codes.InvalidArgument, "invalid collection id: %v", err)
//		}
//	}
//
//	requiredEventEncodingVersion := req.GetEventEncodingVersion()
//
//	result, err := h.accessBackend.GetTransactionResult(context, transactionID, blockId, collectionId, requiredEventEncodingVersion)
//	if err != nil {
//		return nil, err
//	}
//
//	metadata, err := h.buildMetadataResponse()
//	if err != nil {
//		return nil, err
//	}
//
//	return &access.TransactionResultResponse{
//		Status:        entities.TransactionStatus(result.Status),
//		StatusCode:    uint32(result.StatusCode),
//		ErrorMessage:  result.ErrorMessage,
//		Events:        rpcConvert.EventsToMessages(result.Events),
//		BlockId:       result.BlockID[:],
//		TransactionId: result.TransactionID[:],
//		CollectionId:  result.CollectionID[:],
//		BlockHeight:   result.BlockHeight,
//		Metadata:      metadata,
//	}, nil
//
//}
//
//func (h *FlowAccessAPILocalForwarder) GetTransactionResultsByBlockID(context context.Context, req *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
//	blockId, err := rpcConvert.BlockID(req.GetBlockId())
//	if err != nil {
//		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
//	}
//
//	requiredEventEncodingVersion := req.GetEventEncodingVersion()
//
//	metadata, err := h.buildMetadataResponse()
//	if err != nil {
//		return nil, err
//	}
//
//	results, err := h.accessBackend.GetTransactionResultsByBlockID(context, blockId, requiredEventEncodingVersion)
//	if err != nil {
//		return nil, err
//	}
//
//	var txResultsResponse []*access.TransactionResultResponse
//	for _, result := range results {
//		resultResponse := &access.TransactionResultResponse{
//			Status:        entities.TransactionStatus(result.Status),
//			StatusCode:    uint32(result.StatusCode),
//			ErrorMessage:  result.ErrorMessage,
//			Events:        rpcConvert.EventsToMessages(result.Events),
//			BlockId:       result.BlockID[:],
//			TransactionId: result.TransactionID[:],
//			CollectionId:  result.CollectionID[:],
//			BlockHeight:   result.BlockHeight,
//		}
//
//		txResultsResponse = append(txResultsResponse, resultResponse)
//	}
//
//	return &access.TransactionResultsResponse{
//		TransactionResults: txResultsResponse,
//		Metadata:           metadata,
//	}, nil
//
//}
//
//func (h *FlowAccessAPILocalForwarder) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
//	blockId, err := rpcConvert.BlockID(req.GetBlockId())
//	if err != nil {
//		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
//	}
//
//	block, err := h.accessBackend.Blocks.ByID(blockId)
//	if err != nil {
//		return nil, rpc.ConvertStorageError(err)
//	}
//
//	index := req.GetIndex()
//
//	requiredEventEncodingVersion := req.GetEventEncodingVersion()
//
//	result, err := h.accessBackend.GetTransactionResultByIndexFromStorage(
//		context,
//		block,
//		index,
//		requiredEventEncodingVersion,
//	)
//	if err != nil {
//		return nil, err
//	}
//
//	metadata, err := h.buildMetadataResponse()
//	if err != nil {
//		return nil, err
//	}
//
//	return &access.TransactionResultResponse{
//		Status:        entities.TransactionStatus(result.Status),
//		StatusCode:    uint32(result.StatusCode),
//		ErrorMessage:  result.ErrorMessage,
//		Events:        rpcConvert.EventsToMessages(result.Events),
//		BlockId:       result.BlockID[:],
//		TransactionId: result.TransactionID[:],
//		CollectionId:  result.CollectionID[:],
//		BlockHeight:   result.BlockHeight,
//		Metadata:      metadata,
//	}, nil
//}
//
//// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and
//// the end block height (inclusive) that have the given type from storage.
//func (h *FlowAccessAPILocalForwarder) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
//	eventType := req.GetType()
//	startHeight := req.GetStartHeight()
//	endHeight := req.GetEndHeight()
//	requiredEventEncodingVersion := req.GetEventEncodingVersion()
//
//	events, err := h.accessBackend.GetEventsForHeightRange(context, eventType, startHeight, endHeight, requiredEventEncodingVersion)
//	if err != nil {
//		return nil, err
//	}
//
//	resultEvents, err := rpcConvert.BlockEventsToMessages(events)
//	if err != nil {
//		return nil, err
//	}
//
//	metadata, err := h.buildMetadataResponse()
//	if err != nil {
//		return nil, err
//	}
//
//	return &access.EventsResponse{
//		Results:  resultEvents,
//		Metadata: metadata,
//	}, nil
//}
//
//// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type from storage.
//func (h *FlowAccessAPILocalForwarder) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
//	eventType := req.GetType()
//	blockIDs := convert.MessagesToIdentifiers(req.GetBlockIds())
//	requiredEventEncodingVersion := req.GetEventEncodingVersion()
//
//	events, err := h.accessBackend.GetEventsForBlockIDs(context, eventType, blockIDs, requiredEventEncodingVersion)
//	if err != nil {
//		return nil, err
//	}
//
//	resultEvents, err := rpcConvert.BlockEventsToMessages(events)
//	if err != nil {
//		return nil, err
//	}
//
//	metadata, err := h.buildMetadataResponse()
//	if err != nil {
//		return nil, err
//	}
//
//	return &access.EventsResponse{
//		Results:  resultEvents,
//		Metadata: metadata,
//	}, nil
//
//}
//
//// buildMetadataResponse builds and returns the metadata response object.
//func (h *FlowAccessAPILocalForwarder) buildMetadataResponse() (*entities.Metadata, error) {
//	lastFinalizedHeader, err := h.state.Final().Head()
//	if err != nil {
//		return nil, fmt.Errorf("could not get finalized, %w", err)
//	}
//	finalizedBlockId := lastFinalizedHeader.ID()
//	nodeId := h.nodeId
//
//	return &entities.Metadata{
//		LatestFinalizedBlockId: finalizedBlockId[:],
//		LatestFinalizedHeight:  lastFinalizedHeader.Height,
//		NodeId:                 nodeId[:],
//	}, nil
//}
