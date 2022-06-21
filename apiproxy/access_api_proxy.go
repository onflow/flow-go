package apiproxy

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

// NewFlowAccessAPIProxy creates a backend access API that forwards some requests to an upstream node.
// It is used by Observer services, Blockchain Data Service, etc.
// Make sure that this is just for observation and not a staked participant in the flow network.
// This means that observers see a copy of the data but there is no interaction to ensure integrity from the root block.
func NewFlowAccessAPIProxy(accessNodeAddressAndPort flow.IdentityList, timeout time.Duration) (*FlowAccessAPIRouter, error) {
	ret := &FlowAccessAPIRouter{}
	err := ret.upstream.setFlowAccessAPI(accessNodeAddressAndPort, timeout)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// setFlowAccessAPI sets a backend access API that forwards some requests to an upstream node.
// It is used by Observer services, Blockchain Data Service, etc.
// Make sure that this is just for observation and not a staked participant in the flow network.
// This means that observers see a copy of the data but there is no interaction to ensure integrity from the root block.
func (ret *FlowAccessAPIForwarder) setFlowAccessAPI(accessNodeAddressAndPort flow.IdentityList, timeout time.Duration) error {
	ret.timeout = timeout
	ret.ids = accessNodeAddressAndPort
	ret.upstream = make([]access.AccessAPIClient, accessNodeAddressAndPort.Count())
	ret.connections = make([]*grpc.ClientConn, accessNodeAddressAndPort.Count())
	for i, identity := range accessNodeAddressAndPort {
		// Store the faultTolerantClient setup parameters such as address, public, key and timeout, so that
		// we can refresh the API on connection loss
		ret.ids[i] = identity

		// We fail on any single error on startup, so that
		// we identify bootstrapping errors early
		err := ret.reconnectingClient(i)
		if err != nil {
			return err
		}
	}

	ret.roundRobin = 0
	return nil
}

// FlowAccessAPIRouter is a structure that represents the routing proxy algorithm.
// It splits requests between a local and a remote API service.
type FlowAccessAPIRouter struct {
	access.AccessAPIServer
	upstream FlowAccessAPIForwarder
}

// SetLocalAPI sets the local backend that responds to block related calls
// Everything else is forwarded to a selected upstream node
func (h *FlowAccessAPIRouter) SetLocalAPI(local access.AccessAPIServer) {
	h.AccessAPIServer = local
}

// reconnectingClient returns an active client, or
// creates one, if the last one is not ready anymore.
func (h *FlowAccessAPIForwarder) reconnectingClient(i int) error {
	timeout := h.timeout

	if h.connections[i] == nil || h.connections[i].GetState() != connectivity.Ready {
		identity := h.ids[i]
		var connection *grpc.ClientConn
		var err error
		if identity.NetworkPubKey == nil {
			connection, err = grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(), //nolint:staticcheck
				backend.WithClientUnaryInterceptor(timeout))
			if err != nil {
				return err
			}
		} else {
			tlsConfig, err := grpcutils.DefaultClientTLSConfig(identity.NetworkPubKey)
			if err != nil {
				return fmt.Errorf("failed to get default TLS client config using public flow networking key %s %w", identity.NetworkPubKey.String(), err)
			}

			connection, err = grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
				backend.WithClientUnaryInterceptor(timeout))
			if err != nil {
				return fmt.Errorf("cannot connect to %s %w", identity.Address, err)
			}
		}
		connection.Connect()
		time.Sleep(1 * time.Second)
		state := connection.GetState()
		if state != connectivity.Ready && state != connectivity.Connecting {
			return fmt.Errorf("%v", state)
		}
		h.connections[i] = connection
		h.upstream[i] = access.NewAccessAPIClient(connection)
	}

	return nil
}

// faultTolerantClient implements an upstream connection that reconnects on errors
// a reasonable amount of time.
func (h *FlowAccessAPIForwarder) faultTolerantClient() (access.AccessAPIClient, error) {
	if h.upstream == nil || len(h.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method not implemented")
	}

	// Reasoning: A retry count of three gives an acceptable 5% failure ratio from a 37% failure ratio.
	// A bigger number is problematic due to the DNS resolve and connection times,
	// plus the need to log and debug each individual connection failure.
	//
	// This reasoning eliminates the need of making this parameter configurable.
	// The logic works rolling over a single connection as well making clean code.
	const retryMax = 3

	h.lock.Lock()
	defer h.lock.Unlock()

	var err error
	for i := 0; i < retryMax; i++ {
		h.roundRobin++
		h.roundRobin = h.roundRobin % len(h.upstream)
		err = h.reconnectingClient(h.roundRobin)
		if err != nil {
			continue
		}
		state := h.connections[h.roundRobin].GetState()
		if state != connectivity.Ready && state != connectivity.Connecting {
			continue
		}
		return h.upstream[h.roundRobin], nil
	}

	return nil, status.Errorf(codes.Unavailable, err.Error())
}

// Ping pings the service. It is special in the sense that it responds successful,
// only if all underlying services are ready.
func (h *FlowAccessAPIRouter) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return h.AccessAPIServer.Ping(context, req)
}

func (h *FlowAccessAPIRouter) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	return h.AccessAPIServer.GetLatestBlockHeader(context, req)
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	return h.AccessAPIServer.GetBlockHeaderByID(context, req)
}

func (h *FlowAccessAPIRouter) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	return h.AccessAPIServer.GetBlockHeaderByHeight(context, req)
}

func (h *FlowAccessAPIRouter) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	return h.AccessAPIServer.GetLatestBlock(context, req)
}

func (h *FlowAccessAPIRouter) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	return h.AccessAPIServer.GetBlockByID(context, req)
}

func (h *FlowAccessAPIRouter) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	return h.AccessAPIServer.GetBlockByHeight(context, req)
}

func (h *FlowAccessAPIRouter) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	return h.AccessAPIServer.GetCollectionByID(context, req)
}

func (h *FlowAccessAPIRouter) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	return h.upstream.SendTransaction(context, req)
}

func (h *FlowAccessAPIRouter) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	return h.upstream.GetTransaction(context, req)
}

func (h *FlowAccessAPIRouter) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	return h.upstream.GetTransactionResult(context, req)
}

func (h *FlowAccessAPIRouter) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	return h.upstream.GetTransactionResultByIndex(context, req)
}

func (h *FlowAccessAPIRouter) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	return h.upstream.GetAccount(context, req)
}

func (h *FlowAccessAPIRouter) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	return h.upstream.GetAccountAtLatestBlock(context, req)
}

func (h *FlowAccessAPIRouter) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	return h.upstream.GetAccountAtBlockHeight(context, req)
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	return h.upstream.ExecuteScriptAtLatestBlock(context, req)
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	return h.upstream.ExecuteScriptAtBlockID(context, req)
}

func (h *FlowAccessAPIRouter) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	return h.upstream.ExecuteScriptAtBlockHeight(context, req)
}

func (h *FlowAccessAPIRouter) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	return h.upstream.GetEventsForHeightRange(context, req)
}

func (h *FlowAccessAPIRouter) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	return h.upstream.GetEventsForBlockIDs(context, req)
}

func (h *FlowAccessAPIRouter) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	return h.AccessAPIServer.GetNetworkParameters(context, req)
}

func (h *FlowAccessAPIRouter) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return h.AccessAPIServer.GetLatestProtocolStateSnapshot(context, req)
}

func (h *FlowAccessAPIRouter) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	return h.upstream.GetExecutionResultForBlockID(context, req)
}

// FlowAccessAPIForwarder forwards all requests to a set of upstream access nodes or observers
type FlowAccessAPIForwarder struct {
	lock        sync.Mutex
	roundRobin  int
	ids         flow.IdentityList
	upstream    []access.AccessAPIClient
	connections []*grpc.ClientConn
	timeout     time.Duration
}

// Ping pings the service. It is special in the sense that it responds successful,
// only if all underlying services are ready.
func (h *FlowAccessAPIForwarder) Ping(context context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.Ping(context, req)
}

func (h *FlowAccessAPIForwarder) GetLatestBlockHeader(context context.Context, req *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetLatestBlockHeader(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockHeaderByID(context context.Context, req *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetBlockHeaderByID(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockHeaderByHeight(context context.Context, req *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetBlockHeaderByHeight(context, req)
}

func (h *FlowAccessAPIForwarder) GetLatestBlock(context context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetLatestBlock(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockByID(context context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetBlockByID(context, req)
}

func (h *FlowAccessAPIForwarder) GetBlockByHeight(context context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetBlockByHeight(context, req)
}

func (h *FlowAccessAPIForwarder) GetCollectionByID(context context.Context, req *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetCollectionByID(context, req)
}

func (h *FlowAccessAPIForwarder) SendTransaction(context context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.SendTransaction(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransaction(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetTransaction(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResult(context context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetTransactionResult(context, req)
}

func (h *FlowAccessAPIForwarder) GetTransactionResultByIndex(context context.Context, req *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetTransactionResultByIndex(context, req)
}

func (h *FlowAccessAPIForwarder) GetAccount(context context.Context, req *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetAccount(context, req)
}

func (h *FlowAccessAPIForwarder) GetAccountAtLatestBlock(context context.Context, req *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetAccountAtLatestBlock(context, req)
}

func (h *FlowAccessAPIForwarder) GetAccountAtBlockHeight(context context.Context, req *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetAccountAtBlockHeight(context, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtLatestBlock(context context.Context, req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.ExecuteScriptAtLatestBlock(context, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtBlockID(context context.Context, req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.ExecuteScriptAtBlockID(context, req)
}

func (h *FlowAccessAPIForwarder) ExecuteScriptAtBlockHeight(context context.Context, req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.ExecuteScriptAtBlockHeight(context, req)
}

func (h *FlowAccessAPIForwarder) GetEventsForHeightRange(context context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetEventsForHeightRange(context, req)
}

func (h *FlowAccessAPIForwarder) GetEventsForBlockIDs(context context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetEventsForBlockIDs(context, req)
}

func (h *FlowAccessAPIForwarder) GetNetworkParameters(context context.Context, req *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetNetworkParameters(context, req)
}

func (h *FlowAccessAPIForwarder) GetLatestProtocolStateSnapshot(context context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetLatestProtocolStateSnapshot(context, req)
}

func (h *FlowAccessAPIForwarder) GetExecutionResultForBlockID(context context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	// This is a passthrough request
	upstream, err := h.faultTolerantClient()
	if err != nil {
		return nil, err
	}
	return upstream.GetExecutionResultForBlockID(context, req)
}
