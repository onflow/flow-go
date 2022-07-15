package backend

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// DefaultClientTimeout is used when making a GRPC request to a collection node or an execution node
const DefaultClientTimeout = 3 * time.Second

// ConnectionFactory is used to create an access api client
type ConnectionFactory interface {
	GetAccessAPIClient(address string) (access.AccessAPIClient, error)
	InvalidateAccessAPIClient(address string)
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, error)
	InvalidateExecutionAPIClient(address string)
}

type ProxyConnectionFactory struct {
	ConnectionFactory
	targetAddress string
}

func (p *ProxyConnectionFactory) GetAccessAPIClient(address string) (access.AccessAPIClient, error) {
	return p.ConnectionFactory.GetAccessAPIClient(p.targetAddress)
}

func (p *ProxyConnectionFactory) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, error) {
	return p.ConnectionFactory.GetExecutionAPIClient(p.targetAddress)
}

type ConnectionFactoryImpl struct {
	CollectionGRPCPort        uint
	ExecutionGRPCPort         uint
	CollectionNodeGRPCTimeout time.Duration
	ExecutionNodeGRPCTimeout  time.Duration
	ConnectionsCache          *lru.Cache
	CacheSize                 uint
	AccessMetrics             module.AccessMetrics
	Log                       zerolog.Logger
	mutex                     sync.Mutex
}

type ConnectionCacheStore struct {
	ClientConn *grpc.ClientConn
	Address    string
	mutex      *sync.Mutex
}

// createConnection creates new gRPC connections to remote node
func (cf *ConnectionFactoryImpl) createConnection(address string, timeout time.Duration) (*grpc.ClientConn, error) {

	if timeout == 0 {
		timeout = DefaultClientTimeout
	}

	keepaliveParams := keepalive.ClientParameters{
		// how long the client will wait before sending a keepalive to the server if there is no activity
		Time: 10 * time.Second,
		// how long the client will wait for a response from the keepalive before closing
		Timeout: timeout,
	}

	// ClientConn's default KeepAlive on connections is indefinite, assuming the timeout isn't reached
	// The connections should be safe to be persisted and reused
	// https://pkg.go.dev/google.golang.org/grpc#WithKeepaliveParams
	// https://grpc.io/blog/grpc-on-http2/#keeping-connections-alive
	conn, err := grpc.Dial(
		address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepaliveParams),
		WithClientUnaryInterceptor(timeout))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}
	return conn, nil
}

func (cf *ConnectionFactoryImpl) retrieveConnection(grpcAddress string, timeout time.Duration) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var store *ConnectionCacheStore

	cf.mutex.Lock()
	if res, ok := cf.ConnectionsCache.Get(grpcAddress); ok {
		store = res.(*ConnectionCacheStore)
		conn = store.ClientConn
		if cf.AccessMetrics != nil {
			cf.AccessMetrics.ConnectionFromPoolRetrieved()
		}
	} else {
		store = &ConnectionCacheStore{
			ClientConn: nil,
			Address:    grpcAddress,
			mutex:      new(sync.Mutex),
		}
		cf.Log.Debug().Str("grpc_conn_added", grpcAddress).Msg("adding grpc connection to pool")
		cf.ConnectionsCache.Add(grpcAddress, store)
		cf.AccessMetrics.ConnectionAddedToPool()
	}
	cf.mutex.Unlock()
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if conn == nil || conn.GetState() == connectivity.Shutdown {
		var err error
		conn, err = cf.createConnection(grpcAddress, timeout)
		if err != nil {
			return nil, err
		}
		store.ClientConn = conn
		if cf.AccessMetrics != nil {
			cf.AccessMetrics.TotalConnectionsInPool(uint(cf.ConnectionsCache.Len()), cf.CacheSize)
		}
	}
	return conn, nil
}

func (cf *ConnectionFactoryImpl) GetAccessAPIClient(address string) (access.AccessAPIClient, error) {

	grpcAddress, err := getGRPCAddress(address, cf.CollectionGRPCPort)
	if err != nil {
		return nil, err
	}

	conn, err := cf.retrieveConnection(grpcAddress, cf.CollectionNodeGRPCTimeout)
	if err != nil {
		return nil, err
	}

	accessAPIClient := access.NewAccessAPIClient(conn)
	return accessAPIClient, nil
}

func (cf *ConnectionFactoryImpl) InvalidateAccessAPIClient(address string) {
	cf.invalidateAPIClient(address, cf.CollectionGRPCPort)
}

func (cf *ConnectionFactoryImpl) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, error) {

	grpcAddress, err := getGRPCAddress(address, cf.ExecutionGRPCPort)
	if err != nil {
		return nil, err
	}

	conn, err := cf.retrieveConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout)
	if err != nil {
		return nil, err
	}

	executionAPIClient := execution.NewExecutionAPIClient(conn)
	return executionAPIClient, nil
}

func (cf *ConnectionFactoryImpl) InvalidateExecutionAPIClient(address string) {
	cf.invalidateAPIClient(address, cf.ExecutionGRPCPort)
}

func (cf *ConnectionFactoryImpl) invalidateAPIClient(address string, port uint) {
	grpcAddress, _ := getGRPCAddress(address, port)
	if res, ok := cf.ConnectionsCache.Get(grpcAddress); ok {
		store := res.(*ConnectionCacheStore)
		conn := store.ClientConn
		store.mutex.Lock()
		store.ClientConn = nil
		store.mutex.Unlock()
		// allow time for any existing requests to finish before closing the connection
		time.Sleep(DefaultClientTimeout)
		conn.Close()
	}
}

// getExecutionNodeAddress translates flow.Identity address to the GRPC address of the node by switching the port to the
// GRPC port from the libp2p port
func getGRPCAddress(address string, grpcPort uint) (string, error) {
	// split hostname and port
	hostnameOrIP, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	// use the hostname from identity list and port number as the one passed in as argument
	grpcAddress := fmt.Sprintf("%s:%d", hostnameOrIP, grpcPort)

	return grpcAddress, nil
}

func WithClientUnaryInterceptor(timeout time.Duration) grpc.DialOption {

	clientTimeoutInterceptor := func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {

		// create a context that expires after timeout
		ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)

		defer cancel()

		// call the remote GRPC using the short context
		err := invoker(ctxWithTimeout, method, req, reply, cc, opts...)

		return err
	}

	return grpc.WithUnaryInterceptor(clientTimeoutInterceptor)
}
