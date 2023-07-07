package backend

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/onflow/flow-go/module"
)

// DefaultClientTimeout is used when making a GRPC request to a collection node or an execution node
const DefaultClientTimeout = 3 * time.Second

type clientType int

const (
	AccessClient clientType = iota
	ExecutionClient
)

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
	CircuitBreakerConfig      *CircuitBreakerConfig
}

// CircuitBreakerConfig is a configuration struct for the circuit breaker.
//
// Enabled specifies whether the circuit breaker is enabled for collection and execution API clients.
//
// RestoreTimeout specifies the duration after which the circuit breaker will restore the connection to the client
// after closing it due to failures.
//
// MaxFailures specifies the maximum number of failed calls to the client that will cause the circuit breaker
// to close the connection.
//
// MaxRetries specifies the maximum number of retries call to check if connection restored after timeout.
type CircuitBreakerConfig struct {
	Enabled        bool
	RestoreTimeout time.Duration
	MaxFailures    uint32
	MaxRetries     uint32
}

type CachedClient struct {
	ClientConn *grpc.ClientConn
	Address    string
	mutex      sync.Mutex
	timeout    time.Duration
}

// createConnection creates new gRPC connections to remote node
func (cf *ConnectionFactoryImpl) createConnection(address string, timeout time.Duration, clientType clientType) (*grpc.ClientConn, error) {

	if timeout == 0 {
		timeout = DefaultClientTimeout
	}

	keepaliveParams := keepalive.ClientParameters{
		// how long the client will wait before sending a keepalive to the server if there is no activity
		Time: 10 * time.Second,
		// how long the client will wait for a response from the keepalive before closing
		Timeout: timeout,
	}

	var connInterceptors []grpc.UnaryClientInterceptor

	if cf.CircuitBreakerConfig != nil && cf.CircuitBreakerConfig.Enabled {
		connInterceptors = append(connInterceptors, cf.createCircuitBreakerInterceptor())
	} else {
		connInterceptors = append(connInterceptors, cf.createClientInvalidationInterceptor(address, clientType))
	}

	connInterceptors = append(connInterceptors, createClientTimeoutInterceptor(timeout))

	// ClientConn's default KeepAlive on connections is indefinite, assuming the timeout isn't reached
	// The connections should be safe to be persisted and reused
	// https://pkg.go.dev/google.golang.org/grpc#WithKeepaliveParams
	// https://grpc.io/blog/grpc-on-http2/#keeping-connections-alive
	conn, err := grpc.Dial(
		address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(cf.MaxMsgSize))),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithChainUnaryInterceptor(connInterceptors...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}
	return conn, nil
}

func (cf *ConnectionFactoryImpl) retrieveConnection(grpcAddress string, timeout time.Duration, clientType clientType) (*grpc.ClientConn, error) {
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
		conn, err = cf.createConnection(grpcAddress, timeout, clientType)
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
		conn, err = cf.retrieveConnection(grpcAddress, cf.CollectionNodeGRPCTimeout, AccessClient)
		if err != nil {
			return nil, nil, err
		}
		return access.NewAccessAPIClient(conn), &noopCloser{}, err
	}

	conn, err = cf.createConnection(grpcAddress, cf.CollectionNodeGRPCTimeout, AccessClient)
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
		conn, err = cf.retrieveConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout, ExecutionClient)
		if err != nil {
			return nil, nil, err
		}
		return execution.NewExecutionAPIClient(conn), &noopCloser{}, nil
	}

	conn, err = cf.createConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout, ExecutionClient)
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

// createClientInvalidationInterceptor creates a client interceptor for client invalidation. It should only be created
// if the circuit breaker is disabled. If the response from the server indicates an unavailable status, it invalidates
// the corresponding client.
func (cf *ConnectionFactoryImpl) createClientInvalidationInterceptor(
	address string,
	clientType clientType,
) grpc.UnaryClientInterceptor {
	if cf.CircuitBreakerConfig == nil || !cf.CircuitBreakerConfig.Enabled {
		clientInvalidationInterceptor := func(
			ctx context.Context,
			method string,
			req interface{},
			reply interface{},
			cc *grpc.ClientConn,
			invoker grpc.UnaryInvoker,
			opts ...grpc.CallOption,
		) error {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if status.Code(err) == codes.Unavailable {
				switch clientType {
				case AccessClient:
					cf.InvalidateAccessAPIClient(address)
				case ExecutionClient:
					cf.InvalidateExecutionAPIClient(address)
				}
			}

			return err
		}

		return clientInvalidationInterceptor
	}

	return nil
}

// The simplified representation and description of circuit breaker pattern, that used to handle node connectivity:
//
// Circuit Open --> Circuit Half-Open --> Circuit Closed
//      ^                                      |
//      |                                      |
//      +--------------------------------------+
//
// The "Circuit Open" state represents the circuit being open, indicating that the node is not available.
// This state is entered when the number of consecutive failures exceeds the maximum allowed failures.
//
// The "Circuit Half-Open" state represents the circuit transitioning from the open state to the half-open
// state after a configured restore timeout. In this state, the circuit allows a limited number of requests
// to test if the node has recovered.
//
// The "Circuit Closed" state represents the circuit being closed, indicating that the node is available.
// This state is initial or entered when the test requests in the half-open state succeed.

// createCircuitBreakerInterceptor creates a client interceptor for circuit breaker functionality. It should only be
// created if the circuit breaker is enabled. All invocations will go through the circuit breaker to be tracked for
// success or failure of the call.
func (cf *ConnectionFactoryImpl) createCircuitBreakerInterceptor() grpc.UnaryClientInterceptor {
	if cf.CircuitBreakerConfig != nil && cf.CircuitBreakerConfig.Enabled {
		circuitBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			// The restore timeout is defined to automatically return the circuit breaker to the HalfClose state.
			Timeout: cf.CircuitBreakerConfig.RestoreTimeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				// The number of maximum failures is checked before the circuit breaker goes to the Open state.
				return counts.ConsecutiveFailures >= cf.CircuitBreakerConfig.MaxFailures
			},
			MaxRequests: cf.CircuitBreakerConfig.MaxRetries,
		})

		circuitBreakerInterceptor := func(
			ctx context.Context,
			method string,
			req interface{},
			reply interface{},
			cc *grpc.ClientConn,
			invoker grpc.UnaryInvoker,
			opts ...grpc.CallOption,
		) error {
			// The circuit breaker integration occurs here, where all invoked calls to the node pass through the
			// CircuitBreaker.Execute method. This method counts successful and failed invocations, and switches to the
			// "StateOpen" when the maximum failure threshold is reached. When the circuit breaker is in the "StateOpen"
			// it immediately rejects connections and returns without waiting for the call timeout. After the
			// "RestoreTimeout" period elapses, the circuit breaker transitions to the "StateHalfOpen" and attempts the
			// invocation again. If the invocation fails, it returns to the "StateOpen"; otherwise, it transitions to
			// the "StateClosed" and handles invocations as usual.
			_, err := circuitBreaker.Execute(func() (interface{}, error) {
				err := invoker(ctx, method, req, reply, cc, opts...)

				return nil, err
			})
			return err
		}

		return circuitBreakerInterceptor
	}

	return nil
}

// createClientTimeoutInterceptor creates a client interceptor with a context that expires after the timeout.
func createClientTimeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	clientTimeoutInterceptor := func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Create a context that expires after the specified timeout.
		ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Call the remote GRPC using the short context.
		err := invoker(ctxWithTimeout, method, req, reply, cc, opts...)

		return err
	}

	return clientTimeoutInterceptor
}

// WithClientTimeoutOption is a helper function to create a GRPC dial option
// with the specified client timeout interceptor.
func WithClientTimeoutOption(timeout time.Duration) grpc.DialOption {
	return grpc.WithUnaryInterceptor(createClientTimeoutInterceptor(timeout))
}
