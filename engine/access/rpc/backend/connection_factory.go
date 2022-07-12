package backend

import (
	"context"
	"fmt"
	"net"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// the default timeout used when making a GRPC request to a collection node or an execution node
const defaultClientTimeout = 3 * time.Second

// ConnectionFactory is used to create an access api client
type ConnectionFactory interface {
	GetAccessAPIClient(address string) (access.AccessAPIClient, error)
	InvalidateAccessAPIClient(address string) bool
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, error)
	InvalidateExecutionAPIClient(address string) bool
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
}

// createConnection creates new gRPC connections to remote node
func (cf *ConnectionFactoryImpl) createConnection(address string, timeout time.Duration) (*grpc.ClientConn, error) {

	if timeout == 0 {
		timeout = defaultClientTimeout
	}

	// ClientConn's default KeepAlive on connections is indefinite, assuming the timeout isn't reached
	// The connections should be safe to be persisted and reused
	// https://pkg.go.dev/google.golang.org/grpc#WithKeepaliveParams
	// https://grpc.io/blog/grpc-on-http2/#keeping-connections-alive
	conn, err := grpc.Dial(
		address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
		grpc.WithInsecure(), //nolint:staticcheck
		WithClientUnaryInterceptor(timeout))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}
	return conn, nil
}

func (cf *ConnectionFactoryImpl) retrieveConnection(grpcAddress string, timeout time.Duration) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	if res, ok := cf.ConnectionsCache.Get(grpcAddress); ok {
		conn = res.(*grpc.ClientConn)
		if cf.AccessMetrics != nil {
			cf.AccessMetrics.ConnectionFromPoolRetrieved()
		}
	}
	if conn == nil || conn.GetState() != connectivity.Ready {
		// This line ensures that when a connection is renewed, the previously cached connection is evicted and closed
		cf.ConnectionsCache.Remove(grpcAddress)
		var err error
		conn, err = cf.createConnection(grpcAddress, timeout)
		if err != nil {
			return nil, err
		}
		cf.ConnectionsCache.Add(grpcAddress, conn)
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

func (cf *ConnectionFactoryImpl) InvalidateAccessAPIClient(address string) bool {
	grpcAddress, err := getGRPCAddress(address, cf.CollectionGRPCPort)
	if err != nil {
		return true
	}
	return cf.ConnectionsCache.Remove(grpcAddress)
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

func (cf *ConnectionFactoryImpl) InvalidateExecutionAPIClient(address string) bool {
	grpcAddress, err := getGRPCAddress(address, cf.ExecutionGRPCPort)
	if err != nil {
		return true
	}
	return cf.ConnectionsCache.Remove(grpcAddress)
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
