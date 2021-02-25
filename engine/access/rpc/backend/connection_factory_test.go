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
	mock2 "github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine/access/mock"
)

func TestExecutionNodeClientTimeout(t *testing.T)  {
	grcpClientTimeout = 100 * time.Millisecond
	en := new(executionNode)
	en.startExecutionNode(t)
	defer en.stopNode(t)

	req := &execution.PingRequest{}
	resp := execution.PingResponse{}
	en.handler.On("Ping", mock2.Anything, req).After(time.Second).Return(resp, nil)
	connectionFactory := new(ConnectionFactoryImpl)


	connectionFactory.ExecutionGRPCPort = en.port

	client, closer, err := connectionFactory.GetExecutionAPIClient(en.listener.Addr().String())
	assert.NoError(t, err)
	defer closer.Close()

	ctx := context.Background()
	_, err = client.Ping(ctx, req)
	assert.NoError(t, err)
}

type node struct {
	server  *grpc.Server
	listener net.Listener
	port uint
}

func (n *node) setupNode(t *testing.T) {
	n.server = grpc.NewServer()
	listener, err := net.Listen("tcp4", ":0")
	assert.NoError(t, err)
	n.listener = listener
	assert.Eventually(t, func() bool {
		return !strings.HasSuffix(listener.Addr().String(), ":0")
	}, time.Second * 4, 10 * time.Millisecond)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	assert.NoError(t, err)
	portAsUint, err := strconv.ParseUint(port, 10, 32)
	assert.NoError(t, err)
	n.port = uint(portAsUint)
}

func (n *node) startNode(t *testing.T) {
	go func() {
		err := n.server.Serve(n.listener)
		assert.NoError(t, err)
	}()
}

func (n *node) stopNode(t *testing.T) {
	if n.server != nil {
		n.server.Stop()
	}
}

type executionNode struct {
	node
	handler *mock.ExecutionAPIServer
}

func (en *executionNode) startExecutionNode(t *testing.T) {
	en.setupNode(t)
	handler := new(mock.ExecutionAPIServer)
	execution.RegisterExecutionAPIServer(en.server, handler)
	en.handler = handler
	en.startNode(t)
}

func (en *executionNode) stopExecutionNode(t *testing.T) {
	en.stopNode(t)
}

type collectionNode struct {
	node
	handler *mock.AccessAPIServer
}

func (cn *collectionNode) startCollectionNode(t *testing.T) {
	cn.setupNode(t)
	handler := new(mock.AccessAPIServer)
	access.RegisterAccessAPIServer(cn.server, handler)
	cn.handler = handler
	cn.startNode(t)
}

func (cn *collectionNode) stopCollectionNode(t *testing.T) {
	cn.stopNode(t)
}



