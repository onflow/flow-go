package backend

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/mock"
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
	// set the connection pool cache size
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	client, closer, err := proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.NoError(t, err)
	defer closer.Close()

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
	// set the connection pool cache size
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     en.listener.Addr().String(),
	}

	// get an execution API client
	client, closer, err := proxyConnectionFactory.GetExecutionAPIClient("foo")
	assert.NoError(t, err)
	defer closer.Close()

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
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	_, closer, err := proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 1)
	assert.NoError(t, err)
	defer closer.Close()

	var conn *grpc.ClientConn
	res, ok := connectionFactory.ConnectionsCache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.(*grpc.ClientConn)

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
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     en.listener.Addr().String(),
	}

	// get an execution API client
	_, closer, err := proxyConnectionFactory.GetExecutionAPIClient("foo")
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 1)
	assert.NoError(t, err)
	defer closer.Close()

	var conn *grpc.ClientConn
	res, ok := connectionFactory.ConnectionsCache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.(*grpc.ClientConn)

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
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	// create the execution API client
	client, closer, err := connectionFactory.GetExecutionAPIClient(en.listener.Addr().String())
	assert.NoError(t, err)
	defer closer.Close()

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
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	// create the collection API client
	client, closer, err := connectionFactory.GetAccessAPIClient(cn.listener.Addr().String())
	assert.NoError(t, err)
	defer closer.Close()

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
	cache, _ := lru.New(2)
	connectionFactory.ConnectionsCache = cache

	cn1Address := cn1.listener.Addr().String()
	cn2Address := cn2.listener.Addr().String()
	cn3Address := cn3.listener.Addr().String()

	// get a collection API client
	_, closer, err := connectionFactory.GetAccessAPIClient(cn1Address)
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 1)
	assert.NoError(t, err)
	defer closer.Close()

	_, closer, err = connectionFactory.GetAccessAPIClient(cn2Address)
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 2)
	assert.NoError(t, err)
	defer closer.Close()

	_, closer, err = connectionFactory.GetAccessAPIClient(cn1Address)
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 2)
	assert.NoError(t, err)
	defer closer.Close()

	// Expecting to replace cn2 because cn1 was accessed more recently
	_, closer, err = connectionFactory.GetAccessAPIClient(cn3Address)
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 2)
	assert.NoError(t, err)
	defer closer.Close()

	contains1 := connectionFactory.ConnectionsCache.Contains(cn1Address)
	contains2 := connectionFactory.ConnectionsCache.Contains(cn2Address)
	contains3 := connectionFactory.ConnectionsCache.Contains(cn3Address)

	assert.True(t, contains1)
	assert.False(t, contains2)
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
	cache, _ := lru.New(5)
	connectionFactory.ConnectionsCache = cache

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     cn.listener.Addr().String(),
	}

	// get a collection API client
	client, closer, err := proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 1)
	assert.NoError(t, err)
	// close connection to simulate something "going wrong" with our stored connection
	closer.Close()

	// check if key still exists
	assert.True(t, connectionFactory.ConnectionsCache.Contains(proxyConnectionFactory.targetAddress))

	ctx := context.Background()
	// make the call to the collection node (should fail, connection closed)
	resp, err := client.Ping(ctx, req)
	assert.Error(t, err)

	// re-access, should replace stale connection in cache with new one
	client, closer, err = proxyConnectionFactory.GetAccessAPIClient("foo")
	assert.Equal(t, connectionFactory.ConnectionsCache.Len(), 1)

	var conn *grpc.ClientConn
	res, ok := connectionFactory.ConnectionsCache.Get(proxyConnectionFactory.targetAddress)
	assert.True(t, ok)
	conn = res.(*grpc.ClientConn)

	// check if api client can be rebuilt with retrieved connection
	accessAPIClient := access.NewAccessAPIClient(conn)
	ctx = context.Background()
	resp, err = accessAPIClient.Ping(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, resp, expected)
}

// node mocks a flow node that runs a GRPC server
type node struct {
	server   *grpc.Server
	listener net.Listener
	port     uint
}

func (n *node) setupNode(t *testing.T) {
	n.server = grpc.NewServer()
	listener, err := net.Listen("tcp4", ":0")
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
	go func() {
		err := n.server.Serve(n.listener)
		assert.NoError(t, err)
	}()
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
