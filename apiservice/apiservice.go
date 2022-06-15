package apiservice

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/connectivity"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/grpcutils"
)

func NewFlowCachedAccessAPI(accessNodeAddressAndPort flow.IdentityList, timeout time.Duration) (*FlowCachedAccessAPI, error) {
	ret := &FlowCachedAccessAPI{}
	ret.timeout = timeout
	ret.upstream = make([]access.AccessAPIClient, accessNodeAddressAndPort.Count())
	ret.connections = make([]*grpc.ClientConn, accessNodeAddressAndPort.Count())
	for i, identity := range accessNodeAddressAndPort {
		// Store the client setup parameters such as address, public, key and timeout, so that
		// we can refresh the API on connection loss
		ret.ids[i] = identity

		// We fail on any single error on startup, so that
		// we identify bootstrapping errors early
		err := ret.updateClient(i)
		if err != nil {
			return nil, err
		}
	}

	ret.roundRobin = 0
	return ret, nil
}

type FlowCachedAccessAPI struct {
	access.AccessAPIServer
	lock        sync.Mutex
	roundRobin  int
	ids         flow.IdentityList
	upstream    []access.AccessAPIClient
	connections []*grpc.ClientConn
	timeout     time.Duration
}

func (h *FlowCachedAccessAPI) SetLocalAPI(local access.AccessAPIServer) {
	h.AccessAPIServer = local
}

func (h *FlowCachedAccessAPI) updateClient(i int) error {
	timeout := h.timeout

	if h.connections[i] == nil || h.connections[i].GetState() != connectivity.Ready {
		identity := h.ids[i]
		if identity.NetworkPubKey == nil {
			clientRPCConnection, err := grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(), //nolint:staticcheck
				backend.WithClientUnaryInterceptor(timeout))
			if err != nil {
				return err
			}

			h.connections[i] = clientRPCConnection
			h.upstream[i] = access.NewAccessAPIClient(clientRPCConnection)
		} else {
			tlsConfig, err := grpcutils.DefaultClientTLSConfig(identity.NetworkPubKey)
			if err != nil {
				return fmt.Errorf("failed to get default TLS client config using public flow networking key %s %w", identity.NetworkPubKey.String(), err)
			}

			clientRPCConnection, err := grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
				backend.WithClientUnaryInterceptor(timeout))
			if err != nil {
				return err
			}

			h.connections[i] = clientRPCConnection
			h.upstream[i] = access.NewAccessAPIClient(clientRPCConnection)
		}
	}

	return nil
}

func (h *FlowCachedAccessAPI) client() (access.AccessAPIClient, error) {
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method not implemented")
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	var err error
	for i := 0; i < 3; i++ {
		h.roundRobin++
		h.roundRobin = h.roundRobin % len(h.upstream)
		err = h.updateClient(h.roundRobin)
		if err == nil {
			return h.upstream[h.roundRobin], nil
		}
	}

	return nil, err
}

func (h *FlowCachedAccessAPI) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return h.AccessAPIServer.Ping(context, req)
}

func (h *FlowCachedAccessAPI) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	return h.AccessAPIServer.GetLatestBlockHeader(context, req)
}

func (h *FlowCachedAccessAPI) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	return h.AccessAPIServer.GetBlockHeaderByID(context, req)
}

func (h *FlowCachedAccessAPI) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	return h.AccessAPIServer.GetBlockHeaderByHeight(context, req)
}

func (h *FlowCachedAccessAPI) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	return h.AccessAPIServer.GetLatestBlock(context, req)
}

func (h *FlowCachedAccessAPI) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	return h.AccessAPIServer.GetBlockByID(context, req)
}

func (h *FlowCachedAccessAPI) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	return h.AccessAPIServer.GetBlockByHeight(context, req)
}

func (h *FlowCachedAccessAPI) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	return h.AccessAPIServer.GetCollectionByID(context, req)
}

func (h *FlowCachedAccessAPI) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.SendTransaction(context, req)
}

func (h *FlowCachedAccessAPI) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetTransaction(context, req)
}

func (h *FlowCachedAccessAPI) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetTransactionResult(context, req)
}

func (h *FlowCachedAccessAPI) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetTransactionResultByIndex(context, req)
}

func (h *FlowCachedAccessAPI) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetAccount(context, req)
}

func (h *FlowCachedAccessAPI) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetAccountAtLatestBlock(context, req)
}

func (h *FlowCachedAccessAPI) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetAccountAtBlockHeight(context, req)
}

func (h *FlowCachedAccessAPI) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.ExecuteScriptAtLatestBlock(context, req)
}

func (h *FlowCachedAccessAPI) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.ExecuteScriptAtBlockID(context, req)
}

func (h *FlowCachedAccessAPI) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.ExecuteScriptAtBlockHeight(context, req)
}

func (h *FlowCachedAccessAPI) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetEventsForHeightRange(context, req)
}

func (h *FlowCachedAccessAPI) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetEventsForBlockIDs(context, req)
}

func (h *FlowCachedAccessAPI) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return h.AccessAPIServer.GetNetworkParameters(context, req)
}

func (h *FlowCachedAccessAPI) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return h.AccessAPIServer.GetLatestProtocolStateSnapshot(context, req)
}

func (h *FlowCachedAccessAPI) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	// This is a passthrough request
	upstream, err := h.client()
	if err != nil {
		return nil, err
	}
	return upstream.GetExecutionResultForBlockID(context, req)
}
