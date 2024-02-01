package connection

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProxyAccessAPI(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// create a collection node
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	req := &access.PingRequest{}
	expected := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the collection grpc port
	connectionFactory.CollectionGRPCPort = cn.port
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		nil,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	client, conn, err := proxyConnectionFactory.GetAccessAPIClient("foo", nil)
	defer conn.Close()
	assert.NoError(t, err)

	ctx := context.Background()
	// make the call to the collection node
	resp, err := client.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

func TestProxyExecutionAPI(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// create an execution node
	en := new(executionNode)
	en.start(t)
	defer en.stop(t)

	req := &execution.PingRequest{}
	expected := &execution.PingResponse{}
	en.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the execution grpc port
	connectionFactory.ExecutionGRPCPort = en.port

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		nil,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     en.listener.Addr().String(),
	}

	// get an execution API client
	client, _, err := proxyConnectionFactory.GetExecutionAPIClient("foo")
	assert.NoError(t, err)

	ctx := context.Background()
	// make the call to the execution node
	resp, err := client.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

func TestProxyAccessAPIConnectionReuse(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// create a collection node
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	req := &access.PingRequest{}
	expected := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the collection grpc port
	connectionFactory.CollectionGRPCPort = cn.port

	// set the connection pool cache size
	cacheSize := 1
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	_, closer, err := proxyConnectionFactory.GetAccessAPIClient("foo", nil)
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)
	assert.Nil(t, closer.Close())

	var conn *grpc.ClientConn
	res, ok := connectionCache.cache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.ClientConn()

	// check if api client can be rebuilt with retrieved connection
	accessAPIClient := access.NewAccessAPIClient(conn)
	ctx := context.Background()
	resp, err := accessAPIClient.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

func TestProxyExecutionAPIConnectionReuse(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// create an execution node
	en := new(executionNode)
	en.start(t)
	defer en.stop(t)

	req := &execution.PingRequest{}
	expected := &execution.PingResponse{}
	en.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the execution grpc port
	connectionFactory.ExecutionGRPCPort = en.port

	// set the connection pool cache size
	cacheSize := 5
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     en.listener.Addr().String(),
	}

	// get an execution API client
	_, closer, err := proxyConnectionFactory.GetExecutionAPIClient("foo")
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)
	assert.Nil(t, closer.Close())

	var conn *grpc.ClientConn
	res, ok := connectionCache.cache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.ClientConn()

	// check if api client can be rebuilt with retrieved connection
	executionAPIClient := execution.NewExecutionAPIClient(conn)
	ctx := context.Background()
	resp, err := executionAPIClient.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

// TestExecutionNodeClientTimeout tests that the execution API client times out after the timeout duration
func TestExecutionNodeClientTimeout(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	timeout := 10 * time.Millisecond

	// create an execution node
	en := new(executionNode)
	en.start(t)
	defer en.stop(t)

	// setup the handler mock to not respond within the timeout
	req := &execution.PingRequest{}
	resp := &execution.PingResponse{}
	en.handler.On("Ping", testifymock.Anything, req).After(timeout+time.Second).Return(resp, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the execution grpc port
	connectionFactory.ExecutionGRPCPort = en.port
	// set the execution grpc client timeout
	connectionFactory.ExecutionNodeGRPCTimeout = timeout

	// set the connection pool cache size
	cacheSize := 5
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	// create the execution API client
	client, _, err := connectionFactory.GetExecutionAPIClient(en.listener.Addr().String())
	require.NoError(t, err)

	ctx := context.Background()
	// make the call to the execution node
	_, err = client.Ping(ctx, req)

	// assert that the client timed out
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

// TestCollectionNodeClientTimeout tests that the collection API client times out after the timeout duration
func TestCollectionNodeClientTimeout(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	timeout := 10 * time.Millisecond

	// create a collection node
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	// setup the handler mock to not respond within the timeout
	req := &access.PingRequest{}
	resp := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, req).After(timeout+time.Second).Return(resp, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the collection grpc port
	connectionFactory.CollectionGRPCPort = cn.port
	// set the collection grpc client timeout
	connectionFactory.CollectionNodeGRPCTimeout = timeout

	// set the connection pool cache size
	cacheSize := 5
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	// create the collection API client
	client, _, err := connectionFactory.GetAccessAPIClient(cn.listener.Addr().String(), nil)
	assert.NoError(t, err)

	ctx := context.Background()
	// make the call to the execution node
	_, err = client.Ping(ctx, req)

	// assert that the client timed out
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

// TestConnectionPoolFull tests that the LRU cache replaces connections when full
func TestConnectionPoolFull(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// create a collection node
	cn1, cn2, cn3 := new(collectionNode), new(collectionNode), new(collectionNode)
	cn1.start(t)
	cn2.start(t)
	cn3.start(t)
	defer cn1.stop(t)
	defer cn2.stop(t)
	defer cn3.stop(t)

	req := &access.PingRequest{}
	expected := &access.PingResponse{}
	cn1.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)
	cn2.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)
	cn3.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the collection grpc port
	connectionFactory.CollectionGRPCPort = cn1.port

	// set the connection pool cache size
	cacheSize := 2
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	cn1Address := "foo1:123"
	cn2Address := "foo2:123"
	cn3Address := "foo3:123"

	// get a collection API client
	// Create and add first client to cache
	_, _, err = connectionFactory.GetAccessAPIClient(cn1Address, nil)
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)

	// Create and add second client to cache
	_, _, err = connectionFactory.GetAccessAPIClient(cn2Address, nil)
	assert.Equal(t, connectionCache.Len(), 2)
	assert.NoError(t, err)

	// Get the first client from cache.
	_, _, err = connectionFactory.GetAccessAPIClient(cn1Address, nil)
	assert.Equal(t, connectionCache.Len(), 2)
	assert.NoError(t, err)

	// Create and add third client to cache, second client will be removed from cache
	_, _, err = connectionFactory.GetAccessAPIClient(cn3Address, nil)
	assert.Equal(t, connectionCache.Len(), 2)
	assert.NoError(t, err)

	var hostnameOrIP string

	hostnameOrIP, _, err = net.SplitHostPort(cn1Address)
	require.NoError(t, err)
	grpcAddress1 := fmt.Sprintf("%s:%d", hostnameOrIP, connectionFactory.CollectionGRPCPort)

	hostnameOrIP, _, err = net.SplitHostPort(cn2Address)
	require.NoError(t, err)
	grpcAddress2 := fmt.Sprintf("%s:%d", hostnameOrIP, connectionFactory.CollectionGRPCPort)

	hostnameOrIP, _, err = net.SplitHostPort(cn3Address)
	require.NoError(t, err)
	grpcAddress3 := fmt.Sprintf("%s:%d", hostnameOrIP, connectionFactory.CollectionGRPCPort)

	assert.True(t, connectionCache.cache.Contains(grpcAddress1))
	assert.False(t, connectionCache.cache.Contains(grpcAddress2))
	assert.True(t, connectionCache.cache.Contains(grpcAddress3))
}

// TestConnectionPoolStale tests that a new connection will be established if the old one cached is stale
func TestConnectionPoolStale(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// create a collection node
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	req := &access.PingRequest{}
	expected := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, req).Return(expected, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the collection grpc port
	connectionFactory.CollectionGRPCPort = cn.port

	// set the connection pool cache size
	cacheSize := 5
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	client, _, err := proxyConnectionFactory.GetAccessAPIClient("foo", nil)
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)
	// close connection to simulate something "going wrong" with our stored connection
	cachedClient, _ := connectionCache.cache.Get(proxyConnectionFactory.targetAddress)

	cachedClient.Invalidate()
	cachedClient.Close()

	ctx := context.Background()
	// make the call to the collection node (should fail, connection closed)
	_, err = client.Ping(ctx, req)
	assert.Error(t, err)

	// re-access, should replace stale connection in cache with new one
	_, _, _ = proxyConnectionFactory.GetAccessAPIClient("foo", nil)
	assert.Equal(t, connectionCache.Len(), 1)

	var conn *grpc.ClientConn
	res, ok := connectionCache.cache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.ClientConn()

	// check if api client can be rebuilt with retrieved connection
	accessAPIClient := access.NewAccessAPIClient(conn)
	ctx = context.Background()
	resp, err := accessAPIClient.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

// TestExecutionNodeClientClosedGracefully tests the scenario where the execution node client is closed gracefully.
//
// Test Steps:
// - Generate a random number of requests and start goroutines to handle each request.
// - Invalidate the execution API client.
// - Wait for all goroutines to finish.
// - Verify that the number of completed requests matches the number of sent responses.
func TestExecutionNodeClientClosedGracefully(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// Add createExecNode function to recreate it each time for rapid test
	createExecNode := func() (*executionNode, func()) {
		en := new(executionNode)
		en.start(t)
		return en, func() {
			en.stop(t)
		}
	}

	// Add rapid test, to check graceful close on different number of requests
	rapid.Check(t, func(tt *rapid.T) {
		en, closer := createExecNode()
		defer closer()

		// setup the handler mock
		req := &execution.PingRequest{}
		resp := &execution.PingResponse{}
		respSent := atomic.NewUint64(0)
		en.handler.On("Ping", testifymock.Anything, req).Run(func(_ testifymock.Arguments) {
			respSent.Inc()
		}).Return(resp, nil)

		// create the factory
		connectionFactory := new(ConnectionFactoryImpl)
		// set the execution grpc port
		connectionFactory.ExecutionGRPCPort = en.port
		// set the execution grpc client timeout
		connectionFactory.ExecutionNodeGRPCTimeout = time.Second

		// set the connection pool cache size
		cacheSize := 1
		connectionCache, err := NewCache(logger, metrics, cacheSize)
		require.NoError(t, err)

		// set metrics reporting
		connectionFactory.AccessMetrics = metrics
		connectionFactory.Manager = NewManager(
			logger,
			connectionFactory.AccessMetrics,
			connectionCache,
			0,
			CircuitBreakerConfig{},
			grpcutils.NoCompressor,
		)

		clientAddress := en.listener.Addr().String()
		// create the execution API client
		client, _, err := connectionFactory.GetExecutionAPIClient(clientAddress)
		assert.NoError(t, err)

		ctx := context.Background()

		// Generate random number of requests
		nofRequests := rapid.IntRange(10, 100).Draw(tt, "nofRequests").(int)
		reqCompleted := atomic.NewUint64(0)

		var waitGroup sync.WaitGroup

		for i := 0; i < nofRequests; i++ {
			waitGroup.Add(1)

			// call Ping request from different goroutines
			go func() {
				defer waitGroup.Done()
				_, err := client.Ping(ctx, req)

				if err == nil {
					reqCompleted.Inc()
				} else {
					require.Equalf(t, codes.Unavailable, status.Code(err), "unexpected error: %v", err)
				}
			}()
		}

		// Close connection
		// connectionFactory.Manager.Remove(clientAddress)

		waitGroup.Wait()

		assert.Equal(t, reqCompleted.Load(), respSent.Load())
	})
}

// TestEvictingCacheClients tests the eviction of cached clients.
// It verifies that when a client is evicted from the cache, subsequent requests are handled correctly.
//
// Test Steps:
//   - Call the gRPC method Ping
//   - While the request is still in progress, remove the connection
//   - Call the gRPC method GetNetworkParameters on the client immediately after eviction and assert the expected
//     error response.
//   - Wait for the client state to change from "Ready" to "Shutdown", indicating that the client connection was closed.
func TestEvictingCacheClients(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// Create a new collection node for testing
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	// Channels used to synchronize test with grpc calls
	startPing := make(chan struct{})      // notify Ping in progress
	returnFromPing := make(chan struct{}) // notify OK to return from Ping

	// Set up mock handlers for Ping and GetNetworkParameters
	pingReq := &access.PingRequest{}
	pingResp := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, pingReq).Return(
		func(context.Context, *access.PingRequest) *access.PingResponse {
			close(startPing)
			<-returnFromPing // keeps request open until returnFromPing is closed
			return pingResp
		},
		func(context.Context, *access.PingRequest) error { return nil },
	)

	netReq := &access.GetNetworkParametersRequest{}
	netResp := &access.GetNetworkParametersResponse{}
	cn.handler.On("GetNetworkParameters", testifymock.Anything, netReq).Return(netResp, nil)

	// Create the connection factory
	connectionFactory := new(ConnectionFactoryImpl)
	// Set the gRPC port
	connectionFactory.CollectionGRPCPort = cn.port
	// Set the gRPC client timeout
	connectionFactory.CollectionNodeGRPCTimeout = 5 * time.Second
	// Set the connection pool cache size
	cacheSize := 1

	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	// create a non-blocking cache
	connectionCache.cache, err = lru.NewWithEvict[string, *CachedClient](cacheSize, func(_ string, client *CachedClient) {
		go client.Close()
	})
	require.NoError(t, err)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics
	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{},
		grpcutils.NoCompressor,
	)

	clientAddress := cn.listener.Addr().String()
	// Create the execution API client
	client, _, err := connectionFactory.GetAccessAPIClient(clientAddress, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Retrieve the cached client from the cache
	cachedClient, ok := connectionCache.cache.Get(clientAddress)
	require.True(t, ok)

	// wait until the client connection is ready
	require.Eventually(t, func() bool {
		return cachedClient.ClientConn().GetState() == connectivity.Ready
	}, 100*time.Millisecond, 10*time.Millisecond, "client timed out before ready")

	// Schedule the invalidation of the access API client while the Ping call is in progress
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-startPing // wait until Ping is called

		// Invalidate the access API client
		cachedClient.Invalidate()

		// Invalidate marks the connection for closure asynchronously, so give it some time to run
		require.Eventually(t, func() bool {
			return cachedClient.closeRequested.Load()
		}, 100*time.Millisecond, 10*time.Millisecond, "client timed out closing connection")

		// Call a gRPC method on the client, requests should be blocked since the connection is invalidated
		resp, err := client.GetNetworkParameters(ctx, netReq)
		assert.Equal(t, status.Errorf(codes.Unavailable, "the connection to %s was closed", clientAddress), err)
		assert.Nil(t, resp)

		close(returnFromPing) // signal it's ok to return from Ping
	}()

	// Call a gRPC method on the client
	_, err = client.Ping(ctx, pingReq)
	// Check that Ping was called
	cn.handler.AssertCalled(t, "Ping", testifymock.Anything, pingReq)
	assert.NoError(t, err)

	// Wait for the client connection to change state from "Ready" to "Shutdown" as connection was closed.
	require.Eventually(t, func() bool {
		return cachedClient.ClientConn().WaitForStateChange(ctx, connectivity.Ready)
	}, 100*time.Millisecond, 10*time.Millisecond, "client timed out transitioning state")

	assert.Equal(t, connectivity.Shutdown, cachedClient.ClientConn().GetState())
	assert.Equal(t, 0, connectionCache.Len())

	wg.Wait() // wait until the move test routine is done
}

var successCodes = []codes.Code{
	codes.Canceled,
	codes.InvalidArgument,
	codes.NotFound,
	codes.Unimplemented,
	codes.OutOfRange,
}

// TestCircuitBreakerExecutionNode tests the circuit breaker for execution nodes.
func TestCircuitBreakerExecutionNode(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	requestTimeout := 500 * time.Millisecond
	circuitBreakerRestoreTimeout := 1500 * time.Millisecond

	// Create an execution node for testing.
	en := new(executionNode)
	en.start(t)
	defer en.stop(t)

	// Create the connection factory.
	connectionFactory := new(ConnectionFactoryImpl)

	// Set the execution gRPC port.
	connectionFactory.ExecutionGRPCPort = en.port

	// Set the execution gRPC client requestTimeout.
	connectionFactory.ExecutionNodeGRPCTimeout = requestTimeout

	// Set the connection pool cache size.
	cacheSize := 1
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{
			Enabled:        true,
			MaxFailures:    1,
			MaxRequests:    1,
			RestoreTimeout: circuitBreakerRestoreTimeout,
		},
		grpcutils.NoCompressor,
	)

	// Set metrics reporting.
	connectionFactory.AccessMetrics = metrics

	// Create the execution API client.
	client, _, err := connectionFactory.GetExecutionAPIClient(en.listener.Addr().String())
	require.NoError(t, err)

	req := &execution.PingRequest{}
	resp := &execution.PingResponse{}

	// Helper function to make the Ping call to the execution node and measure the duration.
	callAndMeasurePingDuration := func(ctx context.Context) (time.Duration, error) {
		start := time.Now()

		// Make the call to the execution node.
		_, err = client.Ping(ctx, req)
		en.handler.AssertCalled(t, "Ping", testifymock.Anything, req)

		return time.Since(start), err
	}

	t.Run("test different states of the circuit breaker", func(t *testing.T) {
		ctx := context.Background()

		// Set up the handler mock to not respond within the requestTimeout.
		en.handler.On("Ping", testifymock.Anything, req).After(2*requestTimeout).Return(resp, nil)

		// Call and measure the duration for the first invocation.
		duration, err := callAndMeasurePingDuration(ctx)
		assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
		assert.LessOrEqual(t, requestTimeout, duration)

		// Call and measure the duration for the second invocation (circuit breaker state is now "Open").
		duration, err = callAndMeasurePingDuration(ctx)
		assert.Equal(t, gobreaker.ErrOpenState, err)
		assert.Greater(t, requestTimeout, duration)

		// Reset the mock Ping for the next invocation to return response without delay
		en.handler.On("Ping", testifymock.Anything, req).Unset()
		en.handler.On("Ping", testifymock.Anything, req).Return(resp, nil)

		// Wait until the circuit breaker transitions to the "HalfOpen" state.
		time.Sleep(circuitBreakerRestoreTimeout + (500 * time.Millisecond))

		// Call and measure the duration for the third invocation (circuit breaker state is now "HalfOpen").
		duration, err = callAndMeasurePingDuration(ctx)
		assert.Greater(t, requestTimeout, duration)
		assert.Equal(t, nil, err)
	})

	for _, code := range successCodes {
		t.Run(fmt.Sprintf("test error %s treated as a success for circuit breaker ", code.String()), func(t *testing.T) {
			ctx := context.Background()

			en.handler.On("Ping", testifymock.Anything, req).Unset()
			en.handler.On("Ping", testifymock.Anything, req).Return(nil, status.Error(code, code.String()))

			duration, err := callAndMeasurePingDuration(ctx)
			require.Error(t, err)
			require.Equal(t, code, status.Code(err))
			require.Greater(t, requestTimeout, duration)
		})
	}
}

// TestCircuitBreakerCollectionNode tests the circuit breaker for collection nodes.
func TestCircuitBreakerCollectionNode(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	requestTimeout := 500 * time.Millisecond
	circuitBreakerRestoreTimeout := 1500 * time.Millisecond

	// Create a collection node for testing.
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	// Create the connection factory.
	connectionFactory := new(ConnectionFactoryImpl)

	// Set the collection gRPC port.
	connectionFactory.CollectionGRPCPort = cn.port

	// Set the collection gRPC client requestTimeout.
	connectionFactory.CollectionNodeGRPCTimeout = requestTimeout

	// Set the connection pool cache size.
	cacheSize := 1
	connectionCache, err := NewCache(logger, metrics, cacheSize)
	require.NoError(t, err)

	connectionFactory.Manager = NewManager(
		logger,
		connectionFactory.AccessMetrics,
		connectionCache,
		0,
		CircuitBreakerConfig{
			Enabled:        true,
			MaxFailures:    1,
			MaxRequests:    1,
			RestoreTimeout: circuitBreakerRestoreTimeout,
		},
		grpcutils.NoCompressor,
	)

	// Set metrics reporting.
	connectionFactory.AccessMetrics = metrics

	// Create the collection API client.
	client, _, err := connectionFactory.GetAccessAPIClient(cn.listener.Addr().String(), nil)
	assert.NoError(t, err)

	req := &access.PingRequest{}
	resp := &access.PingResponse{}

	// Helper function to make the Ping call to the collection node and measure the duration.
	callAndMeasurePingDuration := func(ctx context.Context) (time.Duration, error) {
		start := time.Now()

		// Make the call to the collection node.
		_, err = client.Ping(ctx, req)
		cn.handler.AssertCalled(t, "Ping", testifymock.Anything, req)

		return time.Since(start), err
	}

	t.Run("test different states of the circuit breaker", func(t *testing.T) {
		ctx := context.Background()

		// Set up the handler mock to not respond within the requestTimeout.
		cn.handler.On("Ping", testifymock.Anything, req).After(2*requestTimeout).Return(resp, nil)

		// Call and measure the duration for the first invocation.
		duration, err := callAndMeasurePingDuration(ctx)
		assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
		assert.LessOrEqual(t, requestTimeout, duration)

		// Call and measure the duration for the second invocation (circuit breaker state is now "Open").
		duration, err = callAndMeasurePingDuration(ctx)
		assert.Equal(t, gobreaker.ErrOpenState, err)
		assert.Greater(t, requestTimeout, duration)

		// Reset the mock Ping for the next invocation to return response without delay
		cn.handler.On("Ping", testifymock.Anything, req).Unset()
		cn.handler.On("Ping", testifymock.Anything, req).Return(resp, nil)

		// Wait until the circuit breaker transitions to the "HalfOpen" state.
		time.Sleep(circuitBreakerRestoreTimeout + (500 * time.Millisecond))

		// Call and measure the duration for the third invocation (circuit breaker state is now "HalfOpen").
		duration, err = callAndMeasurePingDuration(ctx)
		assert.Greater(t, requestTimeout, duration)
		assert.Equal(t, nil, err)
	})

	for _, code := range successCodes {
		t.Run(fmt.Sprintf("test error %s treated as a success for circuit breaker ", code.String()), func(t *testing.T) {
			ctx := context.Background()

			cn.handler.On("Ping", testifymock.Anything, req).Unset()
			cn.handler.On("Ping", testifymock.Anything, req).Return(nil, status.Error(code, code.String()))

			duration, err := callAndMeasurePingDuration(ctx)
			require.Error(t, err)
			require.Equal(t, code, status.Code(err))
			require.Greater(t, requestTimeout, duration)
		})
	}
}
