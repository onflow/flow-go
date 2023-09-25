package connection

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

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
