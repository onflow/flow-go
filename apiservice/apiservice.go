package apiservice

import (
	"context"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"time"
)

func NewFlowAPIService(accessNodeAddressAndPort flow.IdentityList, timeout time.Duration) (*FlowAPIService, error) {
	accessClients := make([]access.AccessAPIClient, accessNodeAddressAndPort.Count())
	for i, identity := range accessNodeAddressAndPort {
		if identity.NetworkPubKey == nil {
			clientRPCConnection, err := grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(), //nolint:staticcheck
				backend.WithClientUnaryInterceptor(timeout))
			if err != nil {
				return nil, err
			}

			accessClients[i] = access.NewAccessAPIClient(clientRPCConnection)
		} else {
			tlsConfig, err := grpcutils.DefaultClientTLSConfig(identity.NetworkPubKey)
			if err != nil {
				return nil, err
			}

			clientRPCConnection, err := grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
				backend.WithClientUnaryInterceptor(timeout))
			if err != nil {
				return nil, err
			}

			accessClients[i] = access.NewAccessAPIClient(clientRPCConnection)
		}
	}

	ret := &FlowAPIService{}
	ret.upstream = accessClients
	ret.LocalCache = nil
	ret.roundRobin = 0
	return ret, nil
}

type FlowAPIService struct {
	access.UnimplementedAccessAPIServer
	roundRobin int
	upstream   []access.AccessAPIClient
	LocalCache access.AccessAPIServer
	downstream *grpc.Server
}

func (h *FlowAPIService) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).Ping(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
	}
	return h.upstream[h.roundRobin].Ping(context, req)
}

func (h FlowAPIService) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetLatestBlockHeader(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetLatestBlockHeader not implemented")
	}
	return h.upstream[h.roundRobin].GetLatestBlockHeader(context, req)
}

func (h FlowAPIService) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetBlockHeaderByID(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetBlockHeaderByID not implemented")
	}
	return h.upstream[h.roundRobin].GetBlockHeaderByID(context, req)
}

func (h FlowAPIService) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetBlockHeaderByHeight(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetBlockHeaderByHeight not implemented")
	}
	return h.upstream[h.roundRobin].GetBlockHeaderByHeight(context, req)
}

func (h FlowAPIService) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetLatestBlock(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetLatestBlock not implemented")
	}
	return h.upstream[h.roundRobin].GetLatestBlock(context, req)
}

func (h FlowAPIService) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetBlockByID(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetBlockByID not implemented")
	}
	return h.upstream[h.roundRobin].GetBlockByID(context, req)
}

func (h FlowAPIService) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetBlockByHeight(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetBlockByHeight not implemented")
	}
	return h.upstream[h.roundRobin].GetBlockByHeight(context, req)
}

func (h FlowAPIService) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetCollectionByID(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetCollectionByID not implemented")
	}
	return h.upstream[h.roundRobin].GetCollectionByID(context, req)
}

func (h FlowAPIService) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method SendTransaction not implemented")
	}
	return h.upstream[h.roundRobin].SendTransaction(context, req)
}

func (h FlowAPIService) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetTransaction not implemented")
	}
	return h.upstream[h.roundRobin].GetTransaction(context, req)
}

func (h FlowAPIService) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetTransactionResult not implemented")
	}
	return h.upstream[h.roundRobin].GetTransactionResult(context, req)
}

func (h FlowAPIService) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetTransactionResultByIndex not implemented")
	}
	return h.upstream[h.roundRobin].GetTransactionResultByIndex(context, req)
}

func (h FlowAPIService) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetAccount not implemented")
	}
	return h.upstream[h.roundRobin].GetAccount(context, req)
}

func (h FlowAPIService) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetAccountAtLatestBlock not implemented")
	}
	return h.upstream[h.roundRobin].GetAccountAtLatestBlock(context, req)
}

func (h FlowAPIService) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetAccountAtBlockHeight not implemented")
	}
	return h.upstream[h.roundRobin].GetAccountAtBlockHeight(context, req)
}

func (h FlowAPIService) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method ExecuteScriptAtLatestBlock not implemented")
	}
	return h.upstream[h.roundRobin].ExecuteScriptAtLatestBlock(context, req)
}

func (h FlowAPIService) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method ExecuteScriptAtBlockID not implemented")
	}
	return h.upstream[h.roundRobin].ExecuteScriptAtBlockID(context, req)
}

func (h FlowAPIService) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method ExecuteScriptAtBlockHeight not implemented")
	}
	return h.upstream[h.roundRobin].ExecuteScriptAtBlockHeight(context, req)
}

func (h FlowAPIService) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetEventsForHeightRange not implemented")
	}
	return h.upstream[h.roundRobin].GetEventsForHeightRange(context, req)
}

func (h FlowAPIService) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetEventsForBlockIDs not implemented")
	}
	return h.upstream[h.roundRobin].GetEventsForBlockIDs(context, req)
}

func (h FlowAPIService) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetNetworkParameters(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetNetworkParameters not implemented")
	}
	return h.upstream[h.roundRobin].GetNetworkParameters(context, req)
}

func (h FlowAPIService) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	if h.LocalCache != nil {
		return (h.LocalCache).GetLatestProtocolStateSnapshot(context, req)
	}
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetLatestProtocolStateSnapshot not implemented")
	}
	return h.upstream[h.roundRobin].GetLatestProtocolStateSnapshot(context, req)
}

func (h FlowAPIService) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	// This is a passthrough request
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method GetExecutionResultForBlockID not implemented")
	}
	return h.upstream[h.roundRobin].GetExecutionResultForBlockID(context, req)
}
