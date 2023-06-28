package backend

import (
	"context"
	"fmt"
	"io"
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
)

// DefaultClientTimeout is used when making a GRPC request to a collection node or an execution node
const DefaultClientTimeout = 3 * time.Second

// ConnectionFactory is used to create an access api client
type ConnectionFactory interface {
	GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error)
	GetAccessAPIClientWithPort(address string, port uint) (access.AccessAPIClient, io.Closer, error)
	InvalidateAccessAPIClient(address string)
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error)
	InvalidateExecutionAPIClient(address string)
}

type ProxyConnectionFactory struct {
	ConnectionFactory
	targetAddress string
}

type noopCloser struct{}

func (c *noopCloser) Close() error {
	return nil
}

func (p *ProxyConnectionFactory) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetAccessAPIClient(p.targetAddress)
}
func (p *ProxyConnectionFactory) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetExecutionAPIClient(p.targetAddress)
}

type ConnectionFactoryImpl struct {
	CollectionGRPCPort        uint
	ExecutionGRPCPort         uint
	CollectionNodeGRPCTimeout time.Duration
	ExecutionNodeGRPCTimeout  time.Duration
	ConnectionsCache          *lru.Cache
	CacheSize                 uint
	MaxMsgSize                uint
	AccessMetrics             module.AccessMetrics
	Log                       zerolog.Logger
	mutex                     sync.Mutex
}

type CachedClient struct {
	ClientConn *grpc.ClientConn
	Address    string
	mutex      sync.Mutex
	timeout    time.Duration
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
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(cf.MaxMsgSize))),
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
	var store *CachedClient
	cacheHit := false
	cf.mutex.Lock()
	if res, ok := cf.ConnectionsCache.Get(grpcAddress); ok {
		cacheHit = true
		store = res.(*CachedClient)
		conn = store.ClientConn
	} else {
		store = &CachedClient{
			ClientConn: nil,
			Address:    grpcAddress,
			timeout:    timeout,
		}
		cf.Log.Debug().Str("cached_client_added", grpcAddress).Msg("adding new cached client to pool")
		cf.ConnectionsCache.Add(grpcAddress, store)
		if cf.AccessMetrics != nil {
			cf.AccessMetrics.ConnectionAddedToPool()
		}
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
			if cacheHit {
				cf.AccessMetrics.ConnectionFromPoolUpdated()
			}
			cf.AccessMetrics.NewConnectionEstablished()
			cf.AccessMetrics.TotalConnectionsInPool(uint(cf.ConnectionsCache.Len()), cf.CacheSize)
		}
	} else if cf.AccessMetrics != nil {
		cf.AccessMetrics.ConnectionFromPoolReused()
	}
	return conn, nil
}

func (cf *ConnectionFactoryImpl) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {
	return cf.GetAccessAPIClientWithPort(address, cf.CollectionGRPCPort)
}

func (cf *ConnectionFactoryImpl) GetAccessAPIClientWithPort(address string, port uint) (access.AccessAPIClient, io.Closer, error) {

	grpcAddress, err := getGRPCAddress(address, port)
	if err != nil {
		return nil, nil, err
	}

	var conn *grpc.ClientConn
	if cf.ConnectionsCache != nil {
		conn, err = cf.retrieveConnection(grpcAddress, cf.CollectionNodeGRPCTimeout)
		if err != nil {
			return nil, nil, err
		}
		return access.NewAccessAPIClient(conn), &noopCloser{}, err
	}

	conn, err = cf.createConnection(grpcAddress, cf.CollectionNodeGRPCTimeout)
	if err != nil {
		return nil, nil, err
	}

	accessAPIClient := access.NewAccessAPIClient(conn)
	closer := io.Closer(conn)
	return accessAPIClient, closer, nil
}

func (cf *ConnectionFactoryImpl) InvalidateAccessAPIClient(address string) {
	if cf.ConnectionsCache != nil {
		cf.Log.Debug().Str("cached_access_client_invalidated", address).Msg("invalidating cached access client")
		cf.invalidateAPIClient(address, cf.CollectionGRPCPort)
	}
}

func (cf *ConnectionFactoryImpl) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {

	grpcAddress, err := getGRPCAddress(address, cf.ExecutionGRPCPort)
	if err != nil {
		return nil, nil, err
	}

	var conn *grpc.ClientConn
	if cf.ConnectionsCache != nil {
		conn, err = cf.retrieveConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout)
		if err != nil {
			return nil, nil, err
		}
		return execution.NewExecutionAPIClient(conn), &noopCloser{}, nil
	}

	conn, err = cf.createConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout)
	if err != nil {
		return nil, nil, err
	}

	executionAPIClient := execution.NewExecutionAPIClient(conn)
	closer := io.Closer(conn)
	return executionAPIClient, closer, nil
}

func (cf *ConnectionFactoryImpl) InvalidateExecutionAPIClient(address string) {
	if cf.ConnectionsCache != nil {
		cf.Log.Debug().Str("cached_execution_client_invalidated", address).Msg("invalidating cached execution client")
		cf.invalidateAPIClient(address, cf.ExecutionGRPCPort)
	}
}

func (cf *ConnectionFactoryImpl) invalidateAPIClient(address string, port uint) {
	grpcAddress, _ := getGRPCAddress(address, port)
	if res, ok := cf.ConnectionsCache.Get(grpcAddress); ok {
		store := res.(*CachedClient)
		store.Close()
		if cf.AccessMetrics != nil {
			cf.AccessMetrics.ConnectionFromPoolInvalidated()
		}
	}
}

func (s *CachedClient) Close() {
	s.mutex.Lock()
	conn := s.ClientConn
	s.ClientConn = nil
	s.mutex.Unlock()
	if conn == nil {
		return
	}
	// allow time for any existing requests to finish before closing the connection
	time.Sleep(s.timeout + 1*time.Second)
	conn.Close()
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
