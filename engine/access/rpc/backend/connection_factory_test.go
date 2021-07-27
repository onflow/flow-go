package backend

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

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
