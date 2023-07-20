package connection

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module"
)

// DefaultClientTimeout is used when making a GRPC request to a collection node or an execution node.
const DefaultClientTimeout = 3 * time.Second

// Manager provides methods for getting and managing gRPC client connections.
type Manager struct {
	cache      *Cache
	logger     zerolog.Logger
	metrics    module.AccessMetrics
	maxMsgSize uint
}

// NewManager creates a new Manager with the specified parameters.
func NewManager(
	cache *Cache,
	logger zerolog.Logger,
	metrics module.AccessMetrics,
	maxMsgSize uint,
) Manager {
	return Manager{
		cache:      cache,
		logger:     logger,
		metrics:    metrics,
		maxMsgSize: maxMsgSize,
	}
}

// GetConnection returns a gRPC client connection for the given grpcAddress and timeout.
// If a cache is used, it retrieves a cached connection, otherwise creates a new connection.
// It returns the client connection and an io.Closer to close the connection when done.
func (m *Manager) GetConnection(grpcAddress string, timeout time.Duration) (*grpc.ClientConn, io.Closer, error) {
	if m.cache != nil {
		conn, err := m.retrieveConnection(grpcAddress, timeout)
		if err != nil {
			return nil, nil, err
		}
		return conn, &noopCloser{}, err
	}

	conn, err := m.createConnection(grpcAddress, timeout, nil)
	if err != nil {
		return nil, nil, err
	}

	return conn, io.Closer(conn), nil
}

// Remove removes the gRPC client connection associated with the given grpcAddress from the cache.
// It returns true if the connection was removed successfully, false otherwise.
func (m *Manager) Remove(grpcAddress string) bool {
	if m.cache == nil {
		return false
	}

	res, ok := m.cache.Get(grpcAddress)
	if !ok {
		return false
	}

	if !m.cache.Remove(grpcAddress) {
		return false
	}

	// Close the connection only if it is successfully removed from the cache
	res.Close()
	return true
}

// HasCache returns true if the Manager has a cache, false otherwise.
func (m *Manager) HasCache() bool {
	return m.cache != nil
}

// retrieveConnection retrieves the CachedClient for the given grpcAddress from the cache or adds a new one if not present.
// If the connection is already cached, it waits for the lock and returns the connection from the cache.
// Otherwise, it creates a new connection and caches it.
func (m *Manager) retrieveConnection(grpcAddress string, timeout time.Duration) (*grpc.ClientConn, error) {
	client, ok := m.cache.GetOrAdd(grpcAddress, timeout)
	if ok {
		// The client was retrieved from the cache, wait for the lock
		client.mu.Lock()
		if m.metrics != nil {
			m.metrics.ConnectionFromPoolReused()
		}
	} else {
		// The client is new, add it to the cache
		if m.metrics != nil {
			m.metrics.ConnectionAddedToPool()
		}
	}
	defer client.mu.Unlock()

	if client.ClientConn != nil && client.ClientConn.GetState() != connectivity.Shutdown {
		// Return the client connection from the cache
		return client.ClientConn, nil
	}

	// The connection is not cached or is closed, create a new connection and cache it
	conn, err := m.createConnection(grpcAddress, timeout, client)
	if err != nil {
		return nil, err
	}

	client.ClientConn = conn
	if m.metrics != nil {
		m.metrics.NewConnectionEstablished()
		m.metrics.TotalConnectionsInPool(uint(m.cache.Len()), uint(m.cache.Size()))
	}

	return client.ClientConn, nil
}

// createConnection creates a new gRPC connection to the remote node at the given address with the specified timeout.
// If the cachedClient is not nil, it means a new entry in the cache is being created, so it's locked to give priority
// to the caller working with the new client, allowing it to create the underlying connection.
func (m *Manager) createConnection(address string, timeout time.Duration, cachedClient *CachedClient) (*grpc.ClientConn, error) {
	if timeout == 0 {
		timeout = DefaultClientTimeout
	}

	keepaliveParams := keepalive.ClientParameters{
		Time:    10 * time.Second, // How long the client will wait before sending a keepalive to the server if there is no activity.
		Timeout: timeout,          // How long the client will wait for a response from the keepalive before closing.
	}

	var connInterceptors []grpc.UnaryClientInterceptor

	// The order in which interceptors are added to the connInterceptors slice is important as they will be called in
	// the same order during gRPC requests. It is crucial to ensure that the request watcher interceptor is added first.
	// This interceptor monitors ongoing requests before passing control to subsequent interceptors.
	if cachedClient != nil {
		connInterceptors = append(connInterceptors, createRequestWatcherInterceptor(cachedClient))
	}

	connInterceptors = append(connInterceptors, createClientTimeoutInterceptor(timeout))

	// ClientConn's default KeepAlive on connections is indefinite, assuming the timeout isn't reached
	// The connections should be safe to be persisted and reused.
	// https://pkg.go.dev/google.golang.org/grpc#WithKeepaliveParams
	// https://grpc.io/blog/grpc-on-http2/#keeping-connections-alive
	conn, err := grpc.Dial(
		address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(m.maxMsgSize))),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithChainUnaryInterceptor(connInterceptors...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}
	return conn, nil
}

// WithClientTimeoutOption is a helper function to create a GRPC dial option with the specified client timeout interceptor.
func WithClientTimeoutOption(timeout time.Duration) grpc.DialOption {
	return grpc.WithUnaryInterceptor(createClientTimeoutInterceptor(timeout))
}

// createRequestWatcherInterceptor creates a request watcher interceptor to wait for unfinished requests before closing.
func createRequestWatcherInterceptor(cachedClient *CachedClient) grpc.UnaryClientInterceptor {
	requestWatcherInterceptor := func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Prevent new requests from being sent if the connection is marked for closure.
		if cachedClient.closeRequested.Load() {
			return status.Errorf(codes.Unavailable, "the connection to %s was closed", cachedClient.Address)
		}

		// Increment the request counter to track ongoing requests, then decrement the request counter before returning.
		cachedClient.wg.Add(1)
		defer cachedClient.wg.Done()

		// Invoke the actual RPC method.
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	return requestWatcherInterceptor
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
