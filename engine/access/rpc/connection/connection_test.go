package connection

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sony/gobreaker"

	"go.uber.org/atomic"
	"pgregory.net/rapid"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProxyAccessAPI(t *testing.T) {
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
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		nil,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	client, conn, err := proxyConnectionFactory.GetAccessAPIClient("foo")
	defer conn.Close()
	assert.NoError(t, err)

	ctx := context.Background()
	// make the call to the collection node
	resp, err := client.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

func TestProxyExecutionAPI(t *testing.T) {
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
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		nil,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
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
	cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
		evictedValue.(*CachedClient).Close()
	})
	connectionCache := NewCache(cache, cacheSize)

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	_, closer, err := proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)
	assert.Nil(t, closer.Close())

	var conn *grpc.ClientConn
	res, ok := connectionCache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.ClientConn

	// check if api client can be rebuilt with retrieved connection
	accessAPIClient := access.NewAccessAPIClient(conn)
	ctx := context.Background()
	resp, err := accessAPIClient.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

func TestProxyExecutionAPIConnectionReuse(t *testing.T) {
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
	cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
		evictedValue.(*CachedClient).Close()
	})
	connectionCache := NewCache(cache, cacheSize)
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
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
	res, ok := connectionCache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.ClientConn

	// check if api client can be rebuilt with retrieved connection
	executionAPIClient := execution.NewExecutionAPIClient(conn)
	ctx := context.Background()
	resp, err := executionAPIClient.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

// TestExecutionNodeClientTimeout tests that the execution API client times out after the timeout duration
func TestExecutionNodeClientTimeout(t *testing.T) {

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
	cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
		evictedValue.(*CachedClient).Close()
	})
	connectionCache := NewCache(cache, cacheSize)
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
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
	cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
		evictedValue.(*CachedClient).Close()
	})
	connectionCache := NewCache(cache, cacheSize)
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
	)

	// create the collection API client
	client, _, err := connectionFactory.GetAccessAPIClient(cn.listener.Addr().String())
	assert.NoError(t, err)

	ctx := context.Background()
	// make the call to the execution node
	_, err = client.Ping(ctx, req)

	// assert that the client timed out
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

// TestConnectionPoolFull tests that the LRU cache replaces connections when full
func TestConnectionPoolFull(t *testing.T) {
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
	cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
		evictedValue.(*CachedClient).Close()
	})
	connectionCache := NewCache(cache, cacheSize)
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
	)

	cn1Address := "foo1:123"
	cn2Address := "foo2:123"
	cn3Address := "foo3:123"

	// get a collection API client
	// Create and add first client to cache
	_, _, err := connectionFactory.GetAccessAPIClient(cn1Address)
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)

	// Create and add second client to cache
	_, _, err = connectionFactory.GetAccessAPIClient(cn2Address)
	assert.Equal(t, connectionCache.Len(), 2)
	assert.NoError(t, err)

	// Peek first client from cache. "recently used"-ness will not be updated, so it will be wiped out first.
	_, _, err = connectionFactory.GetAccessAPIClient(cn1Address)
	assert.Equal(t, connectionCache.Len(), 2)
	assert.NoError(t, err)

	// Create and add third client to cache, firs client will be removed from cache
	_, _, err = connectionFactory.GetAccessAPIClient(cn3Address)
	assert.Equal(t, connectionCache.Len(), 2)
	assert.NoError(t, err)

	var hostnameOrIP string
	hostnameOrIP, _, err = net.SplitHostPort(cn1Address)
	assert.NoError(t, err)
	grpcAddress1 := fmt.Sprintf("%s:%d", hostnameOrIP, connectionFactory.CollectionGRPCPort)
	hostnameOrIP, _, err = net.SplitHostPort(cn2Address)
	assert.NoError(t, err)
	grpcAddress2 := fmt.Sprintf("%s:%d", hostnameOrIP, connectionFactory.CollectionGRPCPort)
	hostnameOrIP, _, err = net.SplitHostPort(cn3Address)
	assert.NoError(t, err)
	grpcAddress3 := fmt.Sprintf("%s:%d", hostnameOrIP, connectionFactory.CollectionGRPCPort)

	contains1 := connectionCache.Contains(grpcAddress1)
	contains2 := connectionCache.Contains(grpcAddress2)
	contains3 := connectionCache.Contains(grpcAddress3)

	assert.False(t, contains1)
	assert.True(t, contains2)
	assert.True(t, contains3)
}

// TestConnectionPoolStale tests that a new connection will be established if the old one cached is stale
func TestConnectionPoolStale(t *testing.T) {
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
	cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
		evictedValue.(*CachedClient).Close()
	})
	connectionCache := NewCache(cache, cacheSize)
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	client, _, err := proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.Equal(t, connectionCache.Len(), 1)
	assert.NoError(t, err)
	// close connection to simulate something "going wrong" with our stored connection
	res, _ := connectionCache.Get(proxyConnectionFactory.targetAddress)

	connectionCache.Remove(proxyConnectionFactory.targetAddress)
	res.Close()

	ctx := context.Background()
	// make the call to the collection node (should fail, connection closed)
	_, err = client.Ping(ctx, req)
	assert.Error(t, err)

	// re-access, should replace stale connection in cache with new one
	_, _, _ = proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.Equal(t, connectionCache.Len(), 1)

	var conn *grpc.ClientConn
	res, ok := connectionCache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.ClientConn

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
	// Add createExecNode function to recreate it each time for rapid test
	createExecNode := func() (*executionNode, func()) {
		en := new(executionNode)
		en.start(t)
		return en, func() {
			en.stop(t)
		}
	}

	// Add rapid test, to check graceful close on different number of requests
	rapid.Check(t, func(t *rapid.T) {
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
		cache, _ := lru.NewWithEvict(cacheSize, func(_, evictedValue interface{}) {
			evictedValue.(*CachedClient).Close()
		})
		connectionCache := NewCache(cache, cacheSize)
		// set metrics reporting
		connectionFactory.AccessMetrics = metrics.NewNoopCollector()
		connectionFactory.Manager = NewManager(
			connectionCache,
			unittest.Logger(),
			connectionFactory.AccessMetrics,
			0,
			CircuitBreakerConfig{},
		)

		clientAddress := en.listener.Addr().String()
		// create the execution API client
		client, _, err := connectionFactory.GetExecutionAPIClient(clientAddress)
		assert.NoError(t, err)

		ctx := context.Background()

		// Generate random number of requests
		nofRequests := rapid.IntRange(10, 100).Draw(t, "nofRequests").(int)
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
		connectionFactory.InvalidateExecutionAPIClient(clientAddress)

		waitGroup.Wait()

		assert.Equal(t, reqCompleted.Load(), respSent.Load())
	})
}

// TestExecutionEvictingCacheClients tests the eviction of cached clients in the execution flow.
// It verifies that when a client is evicted from the cache, subsequent requests are handled correctly.
//
// Test Steps:
//   - Call the gRPC method Ping with a delayed response.
//   - Invalidate the access API client during the Ping call and verify the expected behavior.
//   - Call the gRPC method GetNetworkParameters on the client immediately after eviction and assert the expected
//     error response.
//   - Wait for the client state to change from "Ready" to "Shutdown", indicating that the client connection was closed.
func TestExecutionEvictingCacheClients(t *testing.T) {
	// Create a new collection node for testing
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	// Set up mock handlers for Ping and GetNetworkParameters
	pingReq := &access.PingRequest{}
	pingResp := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, pingReq).After(time.Second).Return(pingResp, nil)

	netReq := &access.GetNetworkParametersRequest{}
	netResp := &access.GetNetworkParametersResponse{}
	cn.handler.On("GetNetworkParameters", testifymock.Anything).Return(netResp, nil)

	// Create the connection factory
	connectionFactory := new(ConnectionFactoryImpl)
	// Set the gRPC port
	connectionFactory.CollectionGRPCPort = cn.port
	// Set the gRPC client timeout
	connectionFactory.CollectionNodeGRPCTimeout = 5 * time.Second
	// Set the connection pool cache size
	cacheSize := 1
	cache, err := lru.New(cacheSize)
	require.NoError(t, err)

	connectionCache := NewCache(cache, cacheSize)
	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		connectionCache,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{},
	)

	clientAddress := cn.listener.Addr().String()
	// Create the execution API client
	client, _, err := connectionFactory.GetAccessAPIClient(clientAddress)
	require.NoError(t, err)

	ctx := context.Background()

	// Retrieve the cached client from the cache
	result, _ := cache.Get(clientAddress)
	cachedClient := result.(*CachedClient)

	// Schedule the invalidation of the access API client after a delay
	time.AfterFunc(250*time.Millisecond, func() {
		// Invalidate the access API client
		connectionFactory.InvalidateAccessAPIClient(clientAddress)

		// Assert that the cached client is marked for closure but still waiting for previous request
		assert.True(t, cachedClient.closeRequested.Load())
		assert.Equal(t, cachedClient.ClientConn.GetState(), connectivity.Ready)

		// Call a gRPC method on the client, that was already evicted
		resp, err := client.GetNetworkParameters(ctx, netReq)
		assert.Equal(t, status.Errorf(codes.Unavailable, "the connection to %s was closed", clientAddress), err)
		assert.Nil(t, resp)
	})

	// Call a gRPC method on the client
	_, err = client.Ping(ctx, pingReq)
	// Check that Ping was called
	cn.handler.AssertCalled(t, "Ping", testifymock.Anything, pingReq)
	assert.NoError(t, err)

	// Wait for the client connection to change state from "Ready" to "Shutdown" as connection was closed.
	changed := cachedClient.ClientConn.WaitForStateChange(ctx, connectivity.Ready)
	assert.True(t, changed)
	assert.Equal(t, connectivity.Shutdown, cachedClient.ClientConn.GetState())
	assert.Equal(t, 0, cache.Len())
}

// TestCircuitBreakerExecutionNode tests the circuit breaker state changes for execution nodes.
func TestCircuitBreakerExecutionNode(t *testing.T) {
	requestTimeout := 500 * time.Millisecond
	circuitBreakerRestoreTimeout := 1500 * time.Millisecond

	// Create an execution node for testing.
	en := new(executionNode)
	en.start(t)
	defer en.stop(t)

	// Set up the handler mock to not respond within the requestTimeout.
	req := &execution.PingRequest{}
	resp := &execution.PingResponse{}
	en.handler.On("Ping", testifymock.Anything, req).After(2*requestTimeout).Return(resp, nil)

	// Create the connection factory.
	connectionFactory := new(ConnectionFactoryImpl)

	// Set the execution gRPC port.
	connectionFactory.ExecutionGRPCPort = en.port

	// Set the execution gRPC client requestTimeout.
	connectionFactory.ExecutionNodeGRPCTimeout = requestTimeout

	// Set the connection pool cache size.
	cacheSize := 1
	connectionCache, _ := lru.New(cacheSize)

	connectionFactory.Manager = NewManager(
		NewCache(connectionCache, cacheSize),
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{
			Enabled:        true,
			MaxFailures:    1,
			MaxRequests:    1,
			RestoreTimeout: circuitBreakerRestoreTimeout,
		},
	)

	// Set metrics reporting.
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()

	// Create the execution API client.
	client, _, err := connectionFactory.GetExecutionAPIClient(en.listener.Addr().String())
	require.NoError(t, err)

	ctx := context.Background()

	// Helper function to make the Ping call to the execution node and measure the duration.
	callAndMeasurePingDuration := func() (time.Duration, error) {
		start := time.Now()

		// Make the call to the execution node.
		_, err = client.Ping(ctx, req)
		en.handler.AssertCalled(t, "Ping", testifymock.Anything, req)

		return time.Since(start), err
	}

	// Call and measure the duration for the first invocation.
	duration, err := callAndMeasurePingDuration()
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	assert.LessOrEqual(t, requestTimeout, duration)

	// Call and measure the duration for the second invocation (circuit breaker state is now "Open").
	duration, err = callAndMeasurePingDuration()
	assert.Equal(t, gobreaker.ErrOpenState, err)
	assert.Greater(t, requestTimeout, duration)

	// Reset the mock Ping for the next invocation to return response without delay
	en.handler.On("Ping", testifymock.Anything, req).Unset()
	en.handler.On("Ping", testifymock.Anything, req).Return(resp, nil)

	// Wait until the circuit breaker transitions to the "HalfOpen" state.
	time.Sleep(circuitBreakerRestoreTimeout + (500 * time.Millisecond))

	// Call and measure the duration for the third invocation (circuit breaker state is now "HalfOpen").
	duration, err = callAndMeasurePingDuration()
	assert.Greater(t, requestTimeout, duration)
	assert.Equal(t, nil, err)
}

// TestCircuitBreakerCollectionNode tests the circuit breaker state changes for collection nodes.
func TestCircuitBreakerCollectionNode(t *testing.T) {
	requestTimeout := 500 * time.Millisecond
	circuitBreakerRestoreTimeout := 1500 * time.Millisecond

	// Create a collection node for testing.
	cn := new(collectionNode)
	cn.start(t)
	defer cn.stop(t)

	// Set up the handler mock to not respond within the requestTimeout.
	req := &access.PingRequest{}
	resp := &access.PingResponse{}
	cn.handler.On("Ping", testifymock.Anything, req).After(2*requestTimeout).Return(resp, nil)

	// Create the connection factory.
	connectionFactory := new(ConnectionFactoryImpl)

	// Set the collection gRPC port.
	connectionFactory.CollectionGRPCPort = cn.port

	// Set the collection gRPC client requestTimeout.
	connectionFactory.CollectionNodeGRPCTimeout = requestTimeout

	// Set the connection pool cache size.
	cacheSize := 1
	connectionCache, _ := lru.New(cacheSize)

	connectionFactory.Manager = NewManager(
		NewCache(connectionCache, cacheSize),
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		0,
		CircuitBreakerConfig{
			Enabled:        true,
			MaxFailures:    1,
			MaxRequests:    1,
			RestoreTimeout: circuitBreakerRestoreTimeout,
		},
	)

	// Set metrics reporting.
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()

	// Create the collection API client.
	client, _, err := connectionFactory.GetAccessAPIClient(cn.listener.Addr().String())
	assert.NoError(t, err)

	ctx := context.Background()

	// Helper function to make the Ping call to the collection node and measure the duration.
	callAndMeasurePingDuration := func() (time.Duration, error) {
		start := time.Now()

		// Make the call to the collection node.
		_, err = client.Ping(ctx, req)
		cn.handler.AssertCalled(t, "Ping", testifymock.Anything, req)

		return time.Since(start), err
	}

	// Call and measure the duration for the first invocation.
	duration, err := callAndMeasurePingDuration()
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	assert.LessOrEqual(t, requestTimeout, duration)

	// Call and measure the duration for the second invocation (circuit breaker state is now "Open").
	duration, err = callAndMeasurePingDuration()
	assert.Equal(t, gobreaker.ErrOpenState, err)
	assert.Greater(t, requestTimeout, duration)

	// Reset the mock Ping for the next invocation to return response without delay
	cn.handler.On("Ping", testifymock.Anything, req).Unset()
	cn.handler.On("Ping", testifymock.Anything, req).Return(resp, nil)

	// Wait until the circuit breaker transitions to the "HalfOpen" state.
	time.Sleep(circuitBreakerRestoreTimeout + (500 * time.Millisecond))

	// Call and measure the duration for the third invocation (circuit breaker state is now "HalfOpen").
	duration, err = callAndMeasurePingDuration()
	assert.Greater(t, requestTimeout, duration)
	assert.Equal(t, nil, err)
}

// node mocks a flow node that runs a GRPC server
type node struct {
	server   *grpc.Server
	listener net.Listener
	port     uint
}

func (n *node) setupNode(t *testing.T) {
	n.server = grpc.NewServer()
	listener, err := net.Listen("tcp4", unittest.DefaultAddress)
	assert.NoError(t, err)
	n.listener = listener
	assert.Eventually(t, func() bool {
		return !strings.HasSuffix(listener.Addr().String(), ":0")
	}, time.Second*4, 10*time.Millisecond)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	assert.NoError(t, err)
	portAsUint, err := strconv.ParseUint(port, 10, 32)
	assert.NoError(t, err)
	n.port = uint(portAsUint)
}

func (n *node) start(t *testing.T) {
	// using a wait group here to ensure the goroutine has started before returning. Otherwise,
	// there's a race condition where the server is sometimes stopped before it has started
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		err := n.server.Serve(n.listener)
		assert.NoError(t, err)
	}()
	unittest.RequireReturnsBefore(t, wg.Wait, 10*time.Millisecond, "could not start goroutine on time")
}

func (n *node) stop(t *testing.T) {
	if n.server != nil {
		n.server.Stop()
	}
}

type executionNode struct {
	node
	handler *mock.ExecutionAPIServer
}

func (en *executionNode) start(t *testing.T) {
	en.setupNode(t)
	handler := new(mock.ExecutionAPIServer)
	execution.RegisterExecutionAPIServer(en.server, handler)
	en.handler = handler
	en.node.start(t)
}

func (en *executionNode) stop(t *testing.T) {
	en.node.stop(t)
}

type collectionNode struct {
	node
	handler *mock.AccessAPIServer
}

func (cn *collectionNode) start(t *testing.T) {
	cn.setupNode(t)
	handler := new(mock.AccessAPIServer)
	access.RegisterAccessAPIServer(cn.server, handler)
	cn.handler = handler
	cn.node.start(t)
}

func (cn *collectionNode) stop(t *testing.T) {
	cn.node.stop(t)
}
